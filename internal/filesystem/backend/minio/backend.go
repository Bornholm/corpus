package minio

import (
	"context"
	"os"
	"strings"

	"github.com/bornholm/corpus/internal/filesystem"
	miniofs "github.com/cpyun/afero-minio"
	"github.com/minio/minio-go/v7"
	"github.com/pkg/errors"
	"github.com/spf13/afero"
)

type Backend struct {
	basePath string
	bucket   string
	client   *minio.Client
}

// Mount implements fs.Backend.
func (b *Backend) Mount(ctx context.Context, fn func(ctx context.Context, fs afero.Fs) error) error {
	fs := miniofs.NewFs(ctx, b.client, b.bucket)

	if err := fn(ctx, afero.NewBasePathFs(fs, b.basePath)); err != nil {
		return errors.WithStack(err)
	}

	return nil
}

func New(client *minio.Client, bucket string, basePath string) *Backend {
	return &Backend{
		bucket:   bucket,
		client:   client,
		basePath: strings.TrimPrefix(basePath, string(os.PathSeparator)),
	}
}

var _ filesystem.Backend = &Backend{}
