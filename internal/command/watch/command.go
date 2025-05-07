package watch

import (
	"context"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/pkg/errors"
	"github.com/progrium/watcher"
	"github.com/redmatter/go-globre/v2"
	"github.com/spf13/afero"
	"github.com/urfave/cli/v2"
	"github.com/urfave/cli/v2/altsrc"

	"github.com/bornholm/corpus/internal/adapter/memory/syncx"
	"github.com/bornholm/corpus/internal/command/common"
	"github.com/bornholm/corpus/internal/core/port"
	"github.com/bornholm/corpus/internal/filesystem"
	"github.com/bornholm/corpus/internal/filesystem/backend"
	"github.com/bornholm/corpus/internal/log"
	"github.com/bornholm/corpus/pkg/client"

	// Filesystem backends

	_ "github.com/bornholm/corpus/internal/filesystem/backend/ftp"
	_ "github.com/bornholm/corpus/internal/filesystem/backend/git"
	_ "github.com/bornholm/corpus/internal/filesystem/backend/local"
	_ "github.com/bornholm/corpus/internal/filesystem/backend/minio"
	_ "github.com/bornholm/corpus/internal/filesystem/backend/sftp"
	_ "github.com/bornholm/corpus/internal/filesystem/backend/smb"
	_ "github.com/bornholm/corpus/internal/filesystem/backend/webdav"
)

func Command() *cli.Command {
	flags := common.WithCommonFlags(
		withWatchFlags()...,
	)
	return &cli.Command{
		Name:   "watch",
		Usage:  "Watch one or more filesystems and automatically index files on change",
		Flags:  flags,
		Before: altsrc.InitInputSourceWithContext(flags, common.NewResolverSourceFromFlagFunc("config")),
		Action: func(ctx *cli.Context) error {
			filesystems, err := getFilesystems(ctx)
			if err != nil {
				return errors.Wrap(err, "could not retrieve filesystems")
			}

			client, err := common.GetCorpusClient(ctx)
			if err != nil {
				return errors.Wrap(err, "could not retrieve corpus client")
			}

			sharedCtx, sharedCancel := context.WithCancel(ctx.Context)
			defer sharedCancel()

			var wg sync.WaitGroup

			wg.Add(len(filesystems))

			for _, f := range filesystems {
				dsn, err := url.Parse(f)
				if err != nil {
					return errors.WithStack(err)
				}

				collections, err := getCorpusCollections(dsn)
				if err != nil {
					return errors.Wrapf(err, "could not retrieve collections from dsn '%s'", dsn)
				}

				source, err := getCorpusSource(dsn)
				if err != nil {
					return errors.Wrapf(err, "could not retrieve source from dsn '%s'", dsn)
				}

				eTagType, err := getCorpusETagType(dsn)
				if err != nil {
					return errors.Wrapf(err, "could not retrieve etag type from dsn '%s'", dsn)
				}

				watchOptions, err := getWatchOptions(dsn)
				if err != nil {
					return errors.Wrapf(err, "could not retrieve watch options from dsn '%s'", dsn)
				}

				b, err := backend.New(dsn.String())
				if err != nil {
					return errors.Wrapf(err, "could not create filesystem backend from dsn '%s'", dsn)
				}

				go func(b filesystem.Backend, dsn *url.URL, collections []string, source *url.URL, watchOptions []filesystem.WatchOptionFunc, eTagType ETagType) {
					defer wg.Done()
					defer sharedCancel()

					watchCtx := log.WithAttrs(sharedCtx, slog.String("filesystem", scrubbedURL(dsn)))

					indexer := &filesystemIndexer{
						collections: collections,
						client:      client,
						backend:     b,
						source:      source,
						eTagType:    eTagType,
					}

					if err := indexer.Watch(watchCtx, watchOptions...); err != nil {
						slog.ErrorContext(watchCtx, "could not watch filesystem", slog.Any("error", errors.WithStack(err)))
					}
				}(b, dsn, collections, source, watchOptions, eTagType)
			}

			wg.Wait()

			return nil
		},
	}
}

const (
	paramCorpusCollections = "corpusCollections"
)

func getCorpusCollections(dsn *url.URL) ([]string, error) {
	query := dsn.Query()

	collections := make([]string, 0)

	if rawCollections := query.Get(paramCorpusCollections); rawCollections != "" {
		colls := strings.Split(rawCollections, ",")
		collections = append(collections, colls...)
		query.Del(paramCorpusCollections)
	}

	dsn.RawQuery = query.Encode()

	return collections, nil
}

const (
	paramCorpusSource = "corpusSource"
)

func getCorpusSource(dsn *url.URL) (*url.URL, error) {
	query := dsn.Query()

	if rawSource := query.Get(paramCorpusSource); rawSource != "" {
		source, err := url.Parse(rawSource)
		if err != nil {
			return nil, errors.WithStack(err)
		}

		query.Del(paramCorpusSource)
		dsn.RawQuery = query.Encode()

		return source, nil
	}

	return nil, nil
}

const (
	paramCorpusETag = "corpusEtag"
)

type ETagType string

const (
	ETagTypeModTime ETagType = "modtime"
	ETagTypeSize    ETagType = "size"
)

var availableETagTypes = []ETagType{
	ETagTypeModTime,
	ETagTypeSize,
}

func getCorpusETagType(dsn *url.URL) (ETagType, error) {
	query := dsn.Query()

	if rawETagType := query.Get(paramCorpusETag); rawETagType != "" {

		eTagType := ETagType(rawETagType)

		if !slices.Contains(availableETagTypes, eTagType) {
			return "", errors.Errorf("could not parse parameter '%s', unexpected value '%s'", paramCorpusETag, rawETagType)
		}

		query.Del(paramCorpusETag)
		dsn.RawQuery = query.Encode()

		return eTagType, nil
	}

	return ETagTypeModTime, nil
}

const (
	paramWatchRecursive = "watchRecursive"
	paramWatchInterval  = "watchInterval"
	paramWatchDirectory = "watchDirectory"
	paramWatchFilter    = "watchFilter"
)

func getWatchOptions(dsn *url.URL) ([]filesystem.WatchOptionFunc, error) {
	options := make([]filesystem.WatchOptionFunc, 0)

	query := dsn.Query()

	recursive := query.Get(paramWatchRecursive)
	switch recursive {
	case "false":
		options = append(options, filesystem.WithRecursive(false))
	case "true":
		fallthrough
	default:
		options = append(options, filesystem.WithRecursive(true))
	}

	query.Del(paramWatchRecursive)

	if rawInterval := query.Get(paramWatchInterval); rawInterval != "" {
		interval, err := time.ParseDuration(rawInterval)
		if err != nil {
			return nil, errors.Wrapf(err, "could not parse '%s' parameter", paramWatchInterval)
		}

		options = append(options, filesystem.WithInterval(interval))
	}

	query.Del(paramWatchInterval)

	if directory := query.Get(paramWatchDirectory); directory != "" {
		options = append(options, filesystem.WithDirectory(directory))
	}

	query.Del(paramWatchDirectory)

	if rawFilter := query.Get(paramWatchFilter); rawFilter != "" {
		pathRegExp := globre.RegexFromGlob(
			rawFilter,
			globre.ExtendedSyntaxEnabled(true),
			globre.GlobStarEnabled(true),
			globre.WithDelimiter('/'),
		)

		filter, err := regexp.Compile(pathRegExp)
		if err != nil {
			return nil, errors.Wrapf(err, "could not parse '%s' parameter", paramWatchFilter)
		}

		options = append(options, filesystem.WithFilter(filter))
	}

	query.Del(paramWatchFilter)

	dsn.RawQuery = query.Encode()

	return options, nil
}

func scrubbedURL(u *url.URL) string {
	if u.User != nil {
		u.User = url.UserPassword("***", "***")
	}
	return u.String()
}

type filesystemIndexer struct {
	client              *client.Client
	collections         []string
	backend             filesystem.Backend
	fs                  afero.Fs
	indexFileDebouncers syncx.Map[string, func(fn func())]
	source              *url.URL
	eTagType            ETagType
}

func (i *filesystemIndexer) Watch(ctx context.Context, funcs ...filesystem.WatchOptionFunc) error {
	funcs = append(funcs, filesystem.WithEvents(
		watcher.Create.String(),
		watcher.Remove.String(),
		watcher.Write.String(),
		watcher.Rename.String(),
	))

	err := i.backend.Mount(ctx, func(ctx context.Context, fs afero.Fs) error {
		slog.InfoContext(ctx, "filesystem mounted")

		defer func() {
			i.fs = nil
		}()

		i.fs = fs

		err := filesystem.Watch(
			ctx, fs,
			i,
			funcs...,
		)
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

// Handle implements filesystem.WatchHandler.
func (i *filesystemIndexer) Handle(ctx context.Context, w *watcher.Watcher, event filesystem.WatchEvent) error {
	if event.IsDir() {
		return nil
	}

	ctx = log.WithAttrs(ctx, slog.String("file", event.Path), slog.String("oldPath", event.OldPath))

	switch event.Op {
	case watcher.Create:
		if err := i.indexFile(ctx, event.Path, event.FileInfo); err != nil {
			slog.ErrorContext(ctx, "could not index file", slog.Any("error", errors.WithStack(err)), slog.String("path", event.Path))
			return nil
		}

	case watcher.Remove:
		if err := i.removeFile(ctx, event.Path, event.FileInfo); err != nil {
			slog.ErrorContext(ctx, "could not remove file", slog.Any("error", errors.WithStack(err)), slog.String("path", event.Path))
			return nil
		}

	case watcher.Write:
		i.indexFileDebounced(ctx, event.Path, event.FileInfo)

	case watcher.Rename:
		if err := i.removeFile(ctx, event.OldPath, event.FileInfo); err != nil {
			slog.ErrorContext(ctx, "could not remove file", slog.Any("error", errors.WithStack(err)), slog.String("path", event.Path))
			return nil
		}
		if err := i.indexFile(ctx, event.Path, event.FileInfo); err != nil {
			slog.ErrorContext(ctx, "could not index file", slog.Any("error", errors.WithStack(err)), slog.String("path", event.Path))
			return nil
		}

	}

	return nil
}

func (i *filesystemIndexer) indexFileDebounced(ctx context.Context, path string, fileInfo os.FileInfo) {
	debounce, _ := i.indexFileDebouncers.LoadOrStore(path, debounced(time.Minute))

	debounce(func() {
		if err := i.indexFile(ctx, path, fileInfo); err != nil {
			slog.ErrorContext(ctx, "could not index file", slog.Any("error", errors.WithStack(err)), slog.String("path", path))
		}

		i.indexFileDebouncers.Delete(path)
	})
}

func (i *filesystemIndexer) indexFile(ctx context.Context, path string, fileInfo os.FileInfo) error {
	source, err := i.getSource(path)
	if err != nil {
		return errors.WithStack(err)
	}

	documents, _, err := i.client.QueryDocuments(ctx, client.WithQueryDocumentsSource(source))
	if err != nil {
		return errors.WithStack(err)
	}

	etag, err := i.getETag(fileInfo)
	if err != nil {
		return errors.WithStack(err)
	}

	if len(documents) > 0 && documents[0].ETag == etag {
		slog.InfoContext(ctx, "document already indexed, skipping", slog.String("etag", etag), slog.String("source", source.String()))
		return nil
	}

	file, err := i.fs.Open(path)
	if err != nil {
		return errors.WithStack(err)
	}

	defer file.Close()

	slog.InfoContext(ctx, "indexing new document")

	task, err := i.client.Index(
		ctx,
		filepath.Base(path), file,
		client.WithIndexSource(source),
		client.WithIndexCollections(i.collections...),
		client.WithIndexETag(etag),
	)
	if err != nil {
		return errors.WithStack(err)
	}

	ctx = log.WithAttrs(ctx, slog.String("taskID", string(task.ID)))

	slog.InfoContext(ctx, "waiting for indexation to complete")

	task, err = i.client.WaitFor(ctx, task.ID)
	if err != nil {
		return errors.WithStack(err)
	}

	if task.Status != port.TaskStatusSucceeded {
		return errors.Errorf("indexation failed: %s (%s)", task.Error, task.Message)
	}

	slog.InfoContext(ctx, "indexation succeeded")

	return nil
}

func (i *filesystemIndexer) removeFile(ctx context.Context, path string, fileInfo os.FileInfo) error {
	source, err := i.getSource(path)
	if err != nil {
		return errors.WithStack(err)
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
