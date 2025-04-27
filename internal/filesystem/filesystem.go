package filesystem

import (
	"context"

	"github.com/pkg/errors"
	"github.com/spf13/afero"
)

type Metadata map[string]any

type FileSystem struct {
	backend  Backend
	metadata Metadata
}

func (fs *FileSystem) Mount(ctx context.Context, fn func(ctx context.Context, fs afero.Fs) error) error {
	if err := fs.backend.Mount(ctx, fn); err != nil {
		return errors.WithStack(err)
	}

	return nil
}

func (fs *FileSystem) Metadata() Metadata {
	return fs.metadata
}

func New(backend Backend, metadata Metadata) *FileSystem {
	return &FileSystem{
		backend:  backend,
		metadata: metadata,
	}
}
