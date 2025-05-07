package git

import (
	"context"
	"crypto/md5"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/bornholm/corpus/internal/filesystem"
	"github.com/bornholm/corpus/internal/log"
	"github.com/bornholm/corpus/internal/util"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
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

	path, err := b.getRepoPath()
	if err != nil {
		return errors.WithStack(err)
	}

	ctx = log.WithAttrs(ctx, slog.String("repoPath", path))

	var ref plumbing.ReferenceName

	if b.branch != "" {
		ref = plumbing.NewBranchReferenceName(b.branch)
	} else {
		ref = plumbing.HEAD
	}

	repo, err := git.PlainCloneContext(ctx, path, false, &git.CloneOptions{
		URL:           b.repoURL,
		Progress:      os.Stderr,
		SingleBranch:  true,
		ReferenceName: ref,
	})
	if err != nil {
		if errors.Is(err, git.ErrRepositoryAlreadyExists) {
			repo, err = git.PlainOpen(path)
			if err != nil {
				return errors.Wrapf(err, "could not open git repository '%s'", path)
			}
		} else {
			return errors.WithStack(err)
		}
	}

	worktree, err := repo.Worktree()
	if err != nil {
		return errors.WithStack(err)
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

				err = worktree.PullContext(ctx, &git.PullOptions{
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

func (b *Backend) getRepoPath() (string, error) {
	repoIdentifier, err := b.getRepoIdentifier()
	if err != nil {
		return "", errors.WithStack(err)
	}

	tmpDir, err := util.TempDir()
	if err != nil {
		return "", errors.WithStack(err)
	}

	return filepath.Join(tmpDir, "git-"+repoIdentifier), nil
}

func (b *Backend) getRepoIdentifier() (string, error) {
	hash := md5.New()

	if _, err := hash.Write([]byte(b.repoURL)); err != nil {
		return "", errors.WithStack(err)
	}

	if _, err := hash.Write([]byte(b.branch)); err != nil {
		return "", errors.WithStack(err)
	}

	sum := hash.Sum(nil)

	return fmt.Sprintf("%x", sum), nil
}

func New(repoURL string, branch string, pullInterval time.Duration) *Backend {
	return &Backend{
		repoURL:      repoURL,
		branch:       branch,
		pullInterval: pullInterval,
	}
}

var _ filesystem.Backend = &Backend{}
