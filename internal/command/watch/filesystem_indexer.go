package watch

import (
	"context"
	"fmt"
	"io/fs"
	"log/slog"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/Bornholm/amatl/pkg/log"
	"github.com/bornholm/corpus/internal/filesystem"
	"github.com/bornholm/corpus/pkg/adapter/memory/syncx"
	"github.com/bornholm/corpus/pkg/client"
	"github.com/bornholm/corpus/pkg/model"
	"github.com/bornholm/go-x/slogx"
	"github.com/pkg/errors"
	"github.com/progrium/watcher"
	"github.com/spf13/afero"
)

type ETagType string

const (
	ETagTypeModTime ETagType = "modtime"
	ETagTypeSize    ETagType = "size"
)

type indexJob struct {
	path     string
	filename string
	fileInfo os.FileInfo
	etag     string
	source   *url.URL
}

type filesystemIndexer struct {
	client               *client.Client
	collections          []model.CollectionID
	backend              filesystem.Backend
	fs                   afero.Fs
	indexFileDebouncers  syncx.Map[string, func(fn func())]
	source               *url.URL
	sourceEmbedded       bool
	eTagType             ETagType
	indexRetryMaxRetries int
	indexRetryBaseDelay  time.Duration
	concurrency          int
	syncOnStart          bool
	deleteOrphans        bool
	toIndex              chan indexJob // set while mounted, nil otherwise
}

func (i *filesystemIndexer) Watch(ctx context.Context, funcs ...filesystem.WatchOptionFunc) error {
	watchOpts := filesystem.NewWatchOptions(funcs...)

	funcs = append(funcs, filesystem.WithEvents(
		watcher.Create.String(),
		watcher.Remove.String(),
		watcher.Write.String(),
		watcher.Rename.String(),
	))

	err := i.backend.Mount(ctx, func(ctx context.Context, afs afero.Fs) error {
		slog.InfoContext(ctx, "filesystem mounted")
		defer func() {
			i.fs = nil
			slog.InfoContext(ctx, "filesystem unmounted")
		}()
		i.fs = afs

		concurrency := i.concurrency
		if concurrency <= 0 {
			concurrency = 8
		}

		toIndex := make(chan indexJob, concurrency*4)
		i.toIndex = toIndex

		workerCtx, cancelWorkers := context.WithCancel(ctx)

		var wg sync.WaitGroup
		for w := 0; w < concurrency; w++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for {
					select {
					case <-workerCtx.Done():
						// drain remaining jobs
						for range toIndex {
						}
						return
					case job, ok := <-toIndex:
						if !ok {
							return
						}
						if err := i.submitIndex(workerCtx, job); err != nil {
							slog.ErrorContext(workerCtx, "could not index file",
								slog.Any("error", errors.WithStack(err)),
								slog.String("path", job.path))
						}
					}
				}
			}()
		}

		if i.syncOnStart {
			if err := i.reconcile(ctx, afs, watchOpts.Directory); err != nil {
				slog.ErrorContext(ctx, "reconciliation failed", slog.Any("error", errors.WithStack(err)))
			}
		}

		err := filesystem.Watch(ctx, afs, i, funcs...)

		cancelWorkers()
		close(toIndex)
		wg.Wait()

		if err != nil {
			return errors.WithStack(err)
		}
		return nil
	})
	if err != nil {
		return errors.WithStack(err)
	}
	return nil
}

func (i *filesystemIndexer) reconcile(ctx context.Context, afs afero.Fs, directory string) error {
	slog.InfoContext(ctx, "starting reconciliation", slog.String("directory", directory))

	// 1. Fetch all indexed digests for this source prefix
	indexed := make(map[string]client.DocumentDigest)

	prefix := i.getSourcePrefix(directory)
	if prefix != "" {
		page := 0
		for {
			digests, err := i.client.ListDocumentDigests(ctx, prefix, page, 500)
			if err != nil {
				return errors.WithStack(err)
			}
			for _, d := range digests {
				indexed[d.Source] = d
			}
			if len(digests) < 500 {
				break
			}
			page++
		}
	}

	slog.InfoContext(ctx, "fetched indexed digests", slog.Int("count", len(indexed)))

	// 2. Walk FS to enumerate local files
	local := make(map[string]indexJob)

	err := afero.Walk(afs, directory, func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			slog.WarnContext(ctx, "walk error", slog.String("path", path), slog.Any("error", err))
			return nil
		}
		if info.IsDir() {
			return nil
		}

		src, err := i.getSource(path)
		if err != nil || src == nil {
			return nil
		}

		etag, err := i.getETag(info)
		if err != nil {
			return nil
		}

		local[src.String()] = indexJob{
			path:     path,
			filename: filepath.Base(path),
			fileInfo: info,
			etag:     etag,
			source:   src,
		}
		return nil
	})
	if err != nil {
		return errors.WithStack(err)
	}

	slog.InfoContext(ctx, "enumerated local files", slog.Int("count", len(local)))

	// 3. Enqueue files to add or update
	toAdd := 0
	for srcStr, job := range local {
		digest, exists := indexed[srcStr]
		if !exists || digest.ETag != job.etag {
			i.toIndex <- job
			toAdd++
		}
	}
	slog.InfoContext(ctx, "enqueued files to index", slog.Int("count", toAdd))

	// 4. Delete orphans
	if i.deleteOrphans {
		toDelete := 0
		for srcStr, digest := range indexed {
			if _, exists := local[srcStr]; !exists {
				slog.InfoContext(ctx, "deleting orphan document",
					slog.String("source", srcStr),
					slog.String("id", string(digest.ID)))
				if err := i.client.DeleteDocument(ctx, digest.ID); err != nil {
					slog.ErrorContext(ctx, "could not delete orphan document",
						slog.Any("error", errors.WithStack(err)),
						slog.String("source", srcStr))
				}
				toDelete++
			}
		}
		slog.InfoContext(ctx, "deleted orphan documents", slog.Int("count", toDelete))
	}

	return nil
}

func (i *filesystemIndexer) getSourcePrefix(directory string) string {
	if i.sourceEmbedded {
		return ""
	}
	src, err := i.getSource(directory)
	if err != nil || src == nil {
		return ""
	}
	s := src.String()
	if !strings.HasSuffix(s, "/") {
		s += "/"
	}
	return s
}

// Handle implements filesystem.WatchHandler.
func (i *filesystemIndexer) Handle(ctx context.Context, w *watcher.Watcher, event filesystem.WatchEvent) error {
	if event.IsDir() {
		return nil
	}

	ctx = slogx.WithAttrs(ctx, slog.String("file", event.Path), slog.String("oldPath", event.OldPath))

	switch event.Op {
	case watcher.Create:
		i.enqueueFile(ctx, event.Path, event.FileInfo)

	case watcher.Remove:
		if err := i.removeFile(ctx, event.Path, event.FileInfo); err != nil {
			slog.ErrorContext(ctx, "could not remove file", slog.Any("error", errors.WithStack(err)), slog.String("path", event.Path))
		}

	case watcher.Write:
		i.indexFileDebounced(ctx, event.Path, event.FileInfo)

	case watcher.Rename:
		if err := i.removeFile(ctx, event.OldPath, event.FileInfo); err != nil {
			slog.ErrorContext(ctx, "could not remove file", slog.Any("error", errors.WithStack(err)), slog.String("path", event.OldPath))
		}
		i.enqueueFile(ctx, event.Path, event.FileInfo)
	}

	return nil
}

func (i *filesystemIndexer) enqueueFile(ctx context.Context, path string, fileInfo os.FileInfo) {
	src, err := i.getSource(path)
	if err != nil {
		slog.ErrorContext(ctx, "could not get source for file", slog.Any("error", errors.WithStack(err)), slog.String("path", path))
		return
	}

	etag, _ := i.getETag(fileInfo)

	job := indexJob{
		path:     path,
		filename: filepath.Base(path),
		fileInfo: fileInfo,
		etag:     etag,
		source:   src,
	}

	if i.toIndex == nil {
		slog.WarnContext(ctx, "index queue not ready, dropping file event", slog.String("path", path))
		return
	}

	select {
	case i.toIndex <- job:
	default:
		slog.WarnContext(ctx, "index queue full, dropping file event", slog.String("path", path))
	}
}

func (i *filesystemIndexer) indexFileDebounced(ctx context.Context, path string, fileInfo os.FileInfo) {
	debounce, _ := i.indexFileDebouncers.LoadOrStore(path, debounced(time.Minute))
	debounce(func() {
		i.enqueueFile(ctx, path, fileInfo)
		i.indexFileDebouncers.Delete(path)
	})
}

func (i *filesystemIndexer) submitIndex(ctx context.Context, job indexJob) error {
	return i.submitIndexWithRetry(ctx, job, 0)
}

func (i *filesystemIndexer) submitIndexWithRetry(ctx context.Context, job indexJob, attempt int) error {
	if i.fs == nil {
		return errors.New("filesystem not mounted")
	}

	file, err := i.fs.Open(job.path)
	if err != nil {
		return errors.WithStack(err)
	}
	defer file.Close()

	opts := []client.IndexOptionFunc{
		client.WithIndexCollections(i.collections...),
	}
	if job.etag != "" {
		opts = append(opts, client.WithIndexETag(job.etag))
	}
	if job.source != nil {
		opts = append(opts, client.WithIndexSource(job.source))
	}

	taskResp, err := i.client.Index(ctx, job.filename, file, opts...)
	if err == nil {
		ctx = log.WithAttrs(ctx, slog.String("taskID", string(taskResp.ID)))
		slog.InfoContext(ctx, "submitted file for indexing", slog.String("path", job.path))
		return nil
	}

	if i.indexRetryMaxRetries == 0 || attempt >= i.indexRetryMaxRetries {
		return errors.Wrapf(err, "submit index failed for %s", job.path)
	}

	backoff := i.indexRetryBaseDelay * time.Duration(1<<attempt)
	slog.WarnContext(ctx, "submit index failed, will retry",
		slog.Any("error", err),
		slog.String("path", job.path),
		slog.Int("attempt", attempt+1),
		slog.Int("maxRetries", i.indexRetryMaxRetries),
		slog.Duration("backoff", backoff))
	time.Sleep(backoff)

	return i.submitIndexWithRetry(ctx, job, attempt+1)
}

func (i *filesystemIndexer) removeFile(ctx context.Context, path string, fileInfo os.FileInfo) error {
	source, err := i.getSource(path)
	if err != nil {
		return errors.WithStack(err)
	}

	if source == nil {
		slog.WarnContext(ctx, "could not retrieve document source, document auto removal unavailable", slog.String("path", path))
		return nil
	}

	ctx = log.WithAttrs(ctx, slog.String("source", source.String()))

	documents, _, err := i.client.QueryDocuments(ctx, client.WithQueryDocumentsSource(source))
	if err != nil {
		return errors.WithStack(err)
	}

	if len(documents) == 0 {
		slog.InfoContext(ctx, "document not found, skipping")
		return nil
	}

	for _, d := range documents {
		slog.InfoContext(ctx, "deleting document", slog.String("documentID", d.ID))
		if err := i.client.DeleteDocument(ctx, d.ID); err != nil {
			slog.ErrorContext(ctx, "could not delete document", slog.Any("error", errors.WithStack(err)))
		}
	}

	return nil
}

const pathMarker = "__PATH__"
const escapedPathMarker = "__ESCAPED_PATH__"

func (i *filesystemIndexer) getSource(path string) (*url.URL, error) {
	if i.sourceEmbedded {
		return nil, nil
	}

	cleanedPath := filepath.Clean(path)

	if i.source != nil {
		escapedPath := url.QueryEscape(filepath.Clean(path))

		rawSource := strings.ReplaceAll(i.source.String(), pathMarker, cleanedPath)
		rawSource = strings.ReplaceAll(rawSource, escapedPathMarker, escapedPath)

		source, err := url.Parse(rawSource)
		if err != nil {
			return nil, errors.WithStack(err)
		}

		return source, nil
	}

	source := &url.URL{
		Scheme: "file",
		Path:   cleanedPath,
	}

	return source, nil
}

func (i *filesystemIndexer) getETag(fileInfo os.FileInfo) (string, error) {
	switch i.eTagType {
	case ETagTypeModTime:
		return fmt.Sprintf("modtime-%d", fileInfo.ModTime().Unix()), nil
	case ETagTypeSize:
		return fmt.Sprintf("size-%d", fileInfo.Size()), nil
	default:
		return "", errors.Errorf("unexpected etag type '%s'", i.eTagType)
	}
}

var _ filesystem.WatchHandler = &filesystemIndexer{}

type debouncer struct {
	mutex sync.Mutex
	timer *time.Timer
	delay time.Duration
}

func debounced(delay time.Duration) func(fn func()) {
	d := &debouncer{
		delay: delay,
	}

	return func(fn func()) {
		d.schedule(fn)
	}
}

func (d *debouncer) schedule(f func()) {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	if d.timer != nil {
		d.timer.Stop()
	}
	d.timer = time.AfterFunc(d.delay, f)
}
