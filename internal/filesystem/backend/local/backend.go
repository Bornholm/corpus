package local

import (
	"context"
	"os"

	"github.com/bornholm/corpus/internal/filesystem"
	"github.com/pkg/errors"
	"github.com/spf13/afero"
)

type Backend struct {
	basePath string
}

// Mount implements fs.Backend.
func (b *Backend) Mount(ctx context.Context, fn func(ctx context.Context, fs afero.Fs) error) error {
	if _, err := os.Stat(b.basePath); err != nil {
		return errors.WithStack(err)
	}

	fs := afero.NewBasePathFs(afero.NewOsFs(), b.basePath)

	if err := fn(ctx, fs); err != nil {
		return errors.WithStack(err)
	}

	return nil
}

func New(basePath string) *Backend {
	return &Backend{
		basePath: basePath,
	}
}

var _ filesystem.Backend = &Backend{}
