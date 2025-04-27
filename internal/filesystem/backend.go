package filesystem

import (
	"context"

	"github.com/spf13/afero"
)

type Backend interface {
	Mount(ctx context.Context, fn func(ctx context.Context, fs afero.Fs) error) error
}
