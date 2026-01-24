package testsuite

import (
	"context"
	"io"
	"log/slog"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/bornholm/corpus/internal/filesystem"
	"github.com/bornholm/corpus/internal/filesystem/backend"
	"github.com/pkg/errors"
	"github.com/progrium/watcher"
	"github.com/spf13/afero"
)

func TestWatch(t *testing.T, dsn string) {
	var done atomic.Bool
	done.Store(false)
	defer done.Store(true)

	t.Logf("Using backend '%s'", dsn)

	backend, err := backend.New(dsn)
	if err != nil {
		t.Fatalf("%+v", errors.WithStack(err))
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var pendingEvents atomic.Int64
	pendingEvents.Store(4)

	defer func() {
		if pe := pendingEvents.Load(); pe > 0 {
			t.Errorf("expected all events to be received, pending %d", pe)
		}
	}()

	err = backend.Mount(ctx, func(ctx context.Context, fs afero.Fs) error {
		fs = filesystem.NewLogger(fs, func(message string, attrs ...slog.Attr) {
			var sb strings.Builder
			sb.WriteString(message)
			sb.WriteString(" ")
			if len(attrs) > 0 {
				sb.WriteString("(")
				for idx, attr := range attrs {
					if idx > 0 {
						sb.WriteString(", ")
					}
					sb.WriteString(attr.String())
				}
				sb.WriteString(")")
			}

			if !done.Load() {
				t.Log(sb.String())
			}
		})

		var handler filesystem.WatchHandlerFunc = func(ctx context.Context, w *watcher.Watcher, event filesystem.WatchEvent) error {
			t.Logf("EVENT: %s -> %s (%s)", event.Op, event.Path, event.OldPath)

			switch event.Path {
			case "watched/1.txt":
				switch event.Op.String() {

				case "CREATE":
					pendingEvents.Add(-1)

				case "WRITE":
					pendingEvents.Add(-1)

					time.Sleep(2 * time.Second)

					file, err := fs.Open(event.Path)
					if err != nil {
						return errors.WithStack(err)
					}

					defer func() {
						if err := file.Close(); err != nil {
							t.Errorf("%+v", errors.WithStack(err))
						}
					}()

					data, err := io.ReadAll(file)
					if err != nil {
						return errors.WithStack(err)
					}

					if e, g := "foo", string(data); e != g {
						t.Errorf("data: expected '%s', got '%s'", e, g)
					}

				default:
					t.Errorf("event.Op: expected 'CREATE', 'WRITE' or 'REMOVE', got '%v'", event.Op.String())
				}

			case "watched/2.txt":
				switch event.Op.String() {

				case "CREATE":
					pendingEvents.Add(-1)

				case "REMOVE":
					pendingEvents.Add(-1)

				case "WRITE":
					// Ignore

				default:
					t.Errorf("event.Op: expected 'CREATE', 'WRITE' or 'REMOVE', got '%v'", event.Op.String())
				}

			}

			if pendingEvents.Load() <= 0 {
				time.Sleep(2 * time.Second)
				cancel()
			}

			return nil
		}

		var wg sync.WaitGroup

		t.Logf("cleaning up watched dir")

		if err := fs.RemoveAll("watched"); err != nil {
			return errors.WithStack(err)
		}

		t.Logf("creating watched dir")

		if err := fs.MkdirAll("watched/subfolder", 0755); err != nil {
			return errors.WithStack(err)
		}

		wg.Add(1)

		go func() {
			defer wg.Done()

			funcs := []filesystem.WatchOptionFunc{
				filesystem.WithInterval(time.Second),
				filesystem.WithDirectory("watched"),
				filesystem.WithEvents("CREATE", "REMOVE", "WRITE"),
			}

			t.Logf("starting watch")

			if err := filesystem.Watch(ctx, fs, handler, funcs...); err != nil {
				t.Errorf("%+v", errors.WithStack(err))
			}
		}()

		time.Sleep(2 * time.Second)

		file, err := fs.Create("watched/1.txt")
		if err != nil {
			return errors.WithStack(err)
		}

		time.Sleep(2 * time.Second)

		if _, err := file.WriteString("foo"); err != nil {
			return errors.WithStack(err)
		}

		if err := file.Close(); err != nil {
			return errors.WithStack(err)
		}

		time.Sleep(2 * time.Second)

		file, err = fs.Create("watched/2.txt")
		if err != nil {
			return errors.WithStack(err)
		}

		if err := file.Close(); err != nil {
			return errors.WithStack(err)
		}

		time.Sleep(2 * time.Second)

		if err := fs.Remove("watched/2.txt"); err != nil {
			return errors.WithStack(err)
		}

		time.Sleep(2 * time.Second)

		wg.Wait()

		return nil
	})
	if err != nil {
		t.Fatalf("%+v", errors.WithStack(err))
	}
}
