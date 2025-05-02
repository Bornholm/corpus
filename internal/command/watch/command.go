package watch

import (
	"context"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
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

			backends := make([]filesystem.Backend, 0, len(filesystems))

			for _, f := range filesystems {
				b, err := backend.New(f)
				if err != nil {
					return errors.Wrapf(err, "could not create filesystem backend from dsn '%s'", f)
				}

				backends = append(backends, b)
			}

			sharedCtx, sharedCancel := context.WithCancel(ctx.Context)
			defer sharedCancel()

			var wg sync.WaitGroup

			wg.Add(len(backends))

			for i, b := range backends {
				dsn, err := url.Parse(filesystems[i])
				if err != nil {
					return errors.WithStack(err)
				}

				collections, err := getCorpusCollections(dsn)
				if err != nil {
					return errors.Wrapf(err, "could not retrieve collections from dsn '%s'", dsn)
				}

				watchOptions, err := getWatchOptions(dsn)
				if err != nil {
					return errors.Wrapf(err, "could not retrieve watch options from dsn '%s'", dsn)
				}

				go func(b filesystem.Backend, dsn *url.URL, collections []string) {
					defer wg.Done()
					defer sharedCancel()

					watchCtx := log.WithAttrs(sharedCtx, slog.String("filesystem", scrubbedURL(dsn)))

					indexer := &filesystemIndexer{
						collections: collections,
						client:      client,
						backend:     b,
					}

					if err := indexer.Watch(watchCtx, watchOptions...); err != nil {
						slog.ErrorContext(watchCtx, "could not watch filesystem", slog.Any("error", errors.WithStack(err)))
					}
				}(b, dsn, collections)
			}

			wg.Wait()

			return nil
		},
	}
}

func getCorpusCollections(dsn *url.URL) ([]string, error) {
	query := dsn.Query()

	collections := make([]string, 0)

	if rawCollections := query.Get("corpusCollections"); rawCollections != "" {
		colls := strings.Split(rawCollections, ",")
		collections = append(collections, colls...)
	}

	return collections, nil
}

func getWatchOptions(dsn *url.URL) ([]filesystem.WatchOptionFunc, error) {
	options := make([]filesystem.WatchOptionFunc, 0)

	query := dsn.Query()

	recursive := query.Get("watchRecursive")
	switch recursive {
	case "false":
		options = append(options, filesystem.WithRecursive(false))
	case "true":
		fallthrough
	default:
		options = append(options, filesystem.WithRecursive(true))
	}

	if rawInterval := query.Get("watchInterval"); rawInterval != "" {
		interval, err := time.ParseDuration(rawInterval)
		if err != nil {
			return nil, errors.Wrap(err, "could not parse 'watchInterval' parameter")
		}

		options = append(options, filesystem.WithInterval(interval))
	}

	if directory := query.Get("watchDirectory"); directory != "" {
		options = append(options, filesystem.WithDirectory(directory))
	}

	if rawFilter := query.Get("watchFilter"); rawFilter != "" {
		pathRegExp := globre.RegexFromGlob(
			rawFilter,
			globre.ExtendedSyntaxEnabled(true),
			globre.GlobStarEnabled(true),
			globre.WithDelimiter('/'),
		)

		filter, err := regexp.Compile(pathRegExp)
		if err != nil {
			return nil, errors.Wrap(err, "could not parse 'watchFilter' parameter")
		}

		options = append(options, filesystem.WithFilter(filter))
	}

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
	source := i.getSource(path)

	documents, _, err := i.client.QueryDocuments(ctx, client.WithQueryDocumentsSource(source))
	if err != nil {
		return errors.WithStack(err)
	}

	etag := i.getETag(fileInfo)

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
	source := i.getSource(path)

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

func (i *filesystemIndexer) getSource(path string) *url.URL {
	source := &url.URL{
		Scheme: "file",
		Path:   filepath.Clean(strings.ReplaceAll(path, " ", "_")),
	}

	return source
}

func (i *filesystemIndexer) getETag(fileInfo os.FileInfo) string {
	return fmt.Sprintf("modtime-%d", fileInfo.ModTime().Unix())
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
