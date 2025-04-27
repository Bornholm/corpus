package testsuite

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/bornholm/corpus/internal/filesystem"
	"github.com/bornholm/corpus/internal/filesystem/backend"
	"github.com/pkg/errors"
	"github.com/progrium/watcher"
	"github.com/spf13/afero"
)

func TestWatch(t *testing.T, dsn string) {
	t.Logf("Using backend '%s'", dsn)
	backend, err := backend.New(dsn)
	if err != nil {
		t.Fatalf("%+v", errors.WithStack(err))
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	expectedEvents := 3

	err = backend.Mount(ctx, func(ctx context.Context, fs afero.Fs) error {
		var handler filesystem.WatchHandlerFunc = func(ctx context.Context, w *watcher.Watcher, event filesystem.WatchEvent) error {
			t.Logf("EVENT: %s -> %s (%s)", event.Op, event.Path, event.OldPath)

			switch event.Path {
			case "watched/1.txt":
				if e, g := "watched/1.txt", event.Path; e != g {
					t.Errorf("event.Path: expected '%v', got '%v'", e, g)
				}

				expectedEvents--

			case "watched/2.txt":
				switch event.Op.String() {

				case "CREATE":
					expectedEvents--

				case "REMOVE":
					if e, g := "watched/2.txt", event.OldPath; e != g {
						t.Errorf("event.OldPath: expected '%v', got '%v'", e, g)
					}

					expectedEvents--

				default:
					t.Errorf("event.Op: expected 'CREATE' or 'REMOVE', got '%v'", event.Op.String())
				}

			}

			if expectedEvents <= 0 {
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

		if err := fs.MkdirAll("watched", 0755); err != nil {
			return errors.WithStack(err)
		}

		wg.Add(1)

		go func() {
			defer wg.Done()

			funcs := []filesystem.WatchOptionFunc{
				filesystem.WithInterval(time.Second),
				filesystem.WithDirectory("watched"),
				filesystem.WithEvents("CREATE", "REMOVE"),
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

		if err := file.Close(); err != nil {
			return errors.WithStack(err)
		}

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

		wg.Wait()

		return nil
	})
	if err != nil {
		t.Fatalf("%+v", errors.WithStack(err))
	}
}
