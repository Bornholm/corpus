package git

import (
	"context"
	"log/slog"
	"os"
	"time"

	"github.com/bornholm/corpus/internal/filesystem"
	"github.com/go-git/go-billy/v5/memfs"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/storage/memory"
	"github.com/pkg/errors"
	"github.com/spf13/afero"
)

type Backend struct {
	repoURL      string
	branch       string
	pullInterval time.Duration
}

// Mount implements fs.Backend.
func (b *Backend) Mount(ctx context.Context, fn func(ctx context.Context, fs afero.Fs) error) error {
	slog.DebugContext(ctx, "cloning repository")

	repo, err := git.CloneContext(ctx, memory.NewStorage(), memfs.New(), &git.CloneOptions{
		URL:      b.repoURL,
		Progress: os.Stderr,
	})
	if err != nil {
		return errors.WithStack(err)
	}

	worktree, err := repo.Worktree()
	if err != nil {
		return errors.WithStack(err)
	}

	var ref plumbing.ReferenceName

	if b.branch != "" {
		ref = plumbing.NewBranchReferenceName(b.branch)
	} else {
		ref = plumbing.HEAD
	}

	err = worktree.PullContext(ctx, &git.PullOptions{
		Force:         true,
		Progress:      os.Stderr,
		ReferenceName: ref,
	})
	if err != nil && !errors.Is(err, git.NoErrAlreadyUpToDate) {
		return errors.WithStack(err)
	}

	ticker := time.NewTicker(b.pullInterval)
	defer ticker.Stop()

	go func() {
		for {
			select {
			case _, ok := <-ticker.C:
				if !ok {
					return
				}

				slog.DebugContext(ctx, "refreshing repository")

				err := worktree.PullContext(ctx, &git.PullOptions{
					Force:         true,
					Progress:      os.Stderr,
					ReferenceName: ref,
				})
				if err != nil && !errors.Is(err, git.NoErrAlreadyUpToDate) {
					slog.ErrorContext(ctx, "could not pull from remote repository", slog.Any("error", errors.WithStack(err)))
				}

			case <-ctx.Done():
				return
			}
		}
	}()

	var fs afero.Fs = NewFs(ctx, repo)

	if err := fn(ctx, fs); err != nil {
		return errors.WithStack(err)
	}

	return nil
}

func New(repoURL string, branch string, pullInterval time.Duration) *Backend {
	return &Backend{
		repoURL:      repoURL,
		branch:       branch,
		pullInterval: pullInterval,
	}
}

var _ filesystem.Backend = &Backend{}
