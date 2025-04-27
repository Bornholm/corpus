package watch

import (
	"context"
	"log/slog"
	"net/url"
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

	"github.com/bornholm/corpus/internal/command/common"
	"github.com/bornholm/corpus/internal/core/port"
	"github.com/bornholm/corpus/internal/filesystem"
	"github.com/bornholm/corpus/internal/filesystem/backend"
	"github.com/bornholm/corpus/internal/log"
	"github.com/bornholm/corpus/pkg/client"

	// Filesystem backends

	_ "github.com/bornholm/corpus/internal/filesystem/backend/ftp"
	_ "github.com/bornholm/corpus/internal/filesystem/backend/local"
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

				collections, err := getIndexCollections(dsn)
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

func getIndexCollections(dsn *url.URL) ([]string, error) {
	query := dsn.Query()

	collections := make([]string, 0)

	if rawCollections := query.Get("watchCollections"); rawCollections != "" {
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
	client      *client.Client
	collections []string
	backend     filesystem.Backend
	fs          afero.Fs
}

func (i *filesystemIndexer) Watch(ctx context.Context, funcs ...filesystem.WatchOptionFunc) error {
	funcs = append(funcs, filesystem.WithEvents(
		watcher.Create.String(),
		watcher.Remove.String(),
		watcher.Write.String(),
		watcher.Rename.String(),
	))

	err := i.backend.Mount(ctx, func(ctx context.Context, fs afero.Fs) error {
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

	switch event.Op {
	case watcher.Create:
		if err := i.indexFile(ctx, event.Path); err != nil {
			slog.ErrorContext(ctx, "could not index file", slog.Any("error", errors.WithStack(err)), slog.String("path", event.Path))
			return nil
		}

	case watcher.Remove:

	case watcher.Write:

	case watcher.Rename:

	}

	return nil
}

func (i *filesystemIndexer) indexFile(ctx context.Context, path string) error {
	file, err := i.fs.Open(path)
	if err != nil {
		return errors.WithStack(err)
	}

	defer file.Close()

	source := &url.URL{
		Scheme: "file",
		Path:   filepath.Clean(strings.ReplaceAll(path, " ", "_")),
	}

	ctx = log.WithAttrs(ctx, slog.String("file", path))

	slog.InfoContext(ctx, "indexing new file")

	task, err := i.client.Index(
		ctx,
		filepath.Base(path), file,
		client.WithIndexSource(source),
		client.WithIndexCollections(i.collections...),
	)
	if err != nil {
		return errors.WithStack(err)
	}

	slog.InfoContext(ctx, "waiting for indexation to complete")

	task, err = i.client.WaitFor(task.ID)
	if err != nil {
		return errors.WithStack(err)
	}

	if task.Status != port.TaskStatusSucceeded {
		return errors.Errorf("indexation failed: %s (%s)", task.Error, task.Message)
	}

	slog.InfoContext(ctx, "indexation suceeded")

	return nil
}

var _ filesystem.WatchHandler = &filesystemIndexer{}
