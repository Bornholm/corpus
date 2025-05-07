package filesystem

import (
	"context"
	"io/fs"
	"log/slog"
	"path/filepath"
	"regexp"
	"slices"
	"time"

	"github.com/pkg/errors"
	"github.com/progrium/watcher"
	"github.com/spf13/afero"
)

type WatchEvent = watcher.Event

type WatchHandler interface {
	Handle(ctx context.Context, w *watcher.Watcher, event WatchEvent) error
}

type WatchHandlerFunc func(ctx context.Context, watcher *watcher.Watcher, event WatchEvent) error

func (f WatchHandlerFunc) Handle(ctx context.Context, watcher *watcher.Watcher, event WatchEvent) error {
	return f(ctx, watcher, event)
}

type WatchOptions struct {
	Events    []string
	Filter    *regexp.Regexp
	Interval  time.Duration
	Directory string
	Recursive bool
}

type WatchOptionFunc func(opts *WatchOptions)

func NewWatchOptions(funcs ...WatchOptionFunc) *WatchOptions {
	opts := &WatchOptions{
		Events: []string{
			watcher.Create.String(),
			watcher.Move.String(),
			watcher.Write.String(),
			watcher.Remove.String(),
			watcher.Rename.String(),
			watcher.Chmod.String(),
		},
		Directory: ".",
		Interval:  time.Second * 30,
		Filter:    nil,
		Recursive: false,
	}
	for _, fn := range funcs {
		fn(opts)
	}
	return opts
}

func WithInterval(interval time.Duration) WatchOptionFunc {
	return func(opts *WatchOptions) {
		opts.Interval = interval
	}
}

func WithDirectory(dir string) WatchOptionFunc {
	return func(opts *WatchOptions) {
		opts.Directory = dir
	}
}

func WithFilter(filter *regexp.Regexp) WatchOptionFunc {
	return func(opts *WatchOptions) {
		opts.Filter = filter
	}
}

func WithEvents(events ...string) WatchOptionFunc {
	return func(opts *WatchOptions) {
		opts.Events = events
	}
}

func WithRecursive(recursive bool) WatchOptionFunc {
	return func(opts *WatchOptions) {
		opts.Recursive = recursive
	}
}

func Watch(ctx context.Context, fs afero.Fs, handler WatchHandler, funcs ...WatchOptionFunc) error {
	opts := NewWatchOptions(funcs...)
	w := watcher.New()

	w.SetFileSystem(fs)

	go func() {
		defer func() {
			go func() {
				w.Close()
				for range w.Event {
				}
				for range w.Error {
				}
			}()
		}()

		eventSet := make(map[string]struct{}, len(opts.Events))
		for _, e := range opts.Events {
			eventSet[e] = struct{}{}
		}

		for {
			select {
			case event, ok := <-w.Event:
				if !ok {
					return
				}

				slog.DebugContext(ctx, "new event", slog.Any("event", event), slog.Time("modTime", event.ModTime()))

				if _, exists := eventSet[event.Op.String()]; !exists {
					slog.DebugContext(ctx, "ignoring event", slog.Any("event", event))

					continue
				}

				go func(event watcher.Event) {
					if err := handler.Handle(ctx, w, event); err != nil {
						slog.ErrorContext(
							ctx, "error while handling event",
							slog.Any("error", errors.WithStack(err)),
						)
					}
				}(event)

				continue

			case err, ok := <-w.Error:
				if !ok {
					return
				}

				slog.ErrorContext(
					ctx, "error while watching files",
					slog.Any("error", errors.WithStack(err)),
				)

				continue

			case <-ctx.Done():
				return
			}
		}
	}()

	if opts.Filter != nil {
		w.AddFilterHook(watcher.RegexFilterHook(opts.Filter, false))
	}

	if opts.Recursive {
		if err := w.AddRecursive(opts.Directory); err != nil {
			return errors.Wrapf(err, "could not add watched recursive directory '%s'", opts.Directory)
		}
	} else {
		if err := w.Add(opts.Directory); err != nil {
			return errors.Wrapf(err, "could not add watched directory '%s'", opts.Directory)
		}
	}

	for path := range w.WatchedFiles() {
		slog.DebugContext(ctx, "watching file", slog.String("path", path))
	}

	hasCreateEvent := slices.Contains(opts.Events, watcher.Create.String())
	if hasCreateEvent {
		go triggerCreateEventForPreExistingFiles(ctx, fs, w, opts.Directory, opts.Filter, opts.Recursive)
	}

	slog.InfoContext(ctx, "starting watcher", slog.Duration("interval", opts.Interval))
	defer slog.InfoContext(ctx, "watcher stopped")

	if err := ctx.Err(); err != nil {
		return errors.WithStack(err)
	}

	// Start the watching process.
	if err := w.Start(opts.Interval); err != nil {
		return errors.Wrap(err, "could not watch files")
	}

	return nil
}

func triggerCreateEventForPreExistingFiles(ctx context.Context, afs afero.Fs, w *watcher.Watcher, directory string, filter *regexp.Regexp, recursive bool) {
	w.Wait()

	slog.InfoContext(ctx, "watcher started, checking for pre-existing files")

	baseDir, err := afs.Open(".")
	if err != nil {
		slog.ErrorContext(ctx, "could not open base directory", slog.Any("error", errors.WithStack(err)))
		return
	}

	defer baseDir.Close()

	// Weird bug workaround: "preload" base directory files to prevent "empty" walk..
	if _, err := baseDir.Readdirnames(-1); err != nil {
		slog.ErrorContext(ctx, "could not list base directory files", slog.Any("error", errors.WithStack(err)))
		return
	}

	err = afero.Walk(afs, directory, func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			return errors.WithStack(err)
		}

		if info.IsDir() {
			return nil
		}

		if !recursive && filepath.Dir(path) != filepath.Clean(directory) {
			slog.DebugContext(ctx, "recursive mode disabled, ignoring file", slog.String("path", path))
			return nil
		}

		slog.DebugContext(ctx, "checking file", slog.String("path", path), slog.Any("filter", filter))

		if filter != nil && !filter.MatchString(path) {
			return nil
		}

		stat, err := afs.Stat(path)
		if err != nil {
			slog.ErrorContext(ctx, "could not stat file", slog.Any("error", errors.WithStack(err)))

			return nil
		}

		slog.InfoContext(ctx, "triggering create event for pre-existing file", slog.String("path", path))

		w.Event <- watcher.Event{Op: watcher.Create, Path: path, FileInfo: stat}

		return nil
	})
	if err != nil {
		slog.ErrorContext(ctx, "could not check pre-existing files", slog.Any("error", errors.WithStack(err)))
		return
	}

	slog.InfoContext(ctx, "done checking for pre-existing files")
}
