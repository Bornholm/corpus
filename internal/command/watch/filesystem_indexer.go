package watch

import (
	"context"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/bornholm/corpus/internal/adapter/memory/syncx"
	"github.com/bornholm/corpus/internal/core/port"
	"github.com/bornholm/corpus/internal/filesystem"
	"github.com/bornholm/corpus/internal/log"
	"github.com/bornholm/corpus/pkg/client"
	"github.com/pkg/errors"
	"github.com/progrium/watcher"
	"github.com/spf13/afero"
)

type ETagType string

const (
	ETagTypeModTime ETagType = "modtime"
	ETagTypeSize    ETagType = "size"
)

type filesystemIndexer struct {
	client              *client.Client
	collections         []string
	backend             filesystem.Backend
	fs                  afero.Fs
	indexFileDebouncers syncx.Map[string, func(fn func())]
	source              *url.URL
	eTagType            ETagType
	semaphore           chan struct{}
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
	i.semaphore <- struct{}{}
	defer func() {
		<-i.semaphore
	}()

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
	i.semaphore <- struct{}{}
	defer func() {
		<-i.semaphore
	}()

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
