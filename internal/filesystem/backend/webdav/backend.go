package webdav

import (
	"context"
	"net/http"
	"sync"
	"time"

	"github.com/bornholm/corpus/internal/filesystem"
	"github.com/pkg/errors"
	"github.com/spf13/afero"
	"github.com/studio-b12/gowebdav"
)

type Backend struct {
	url    string
	config *Config
	pool   sync.Pool
}

type Config struct {
	Username string
	Password string
	Timeout  time.Duration
}

// Mount implements fs.Backend.
func (b *Backend) Mount(ctx context.Context, fn func(ctx context.Context, fs afero.Fs) error) error {
	withClient := func(fn func(client *gowebdav.Client) error) error {
		client, ok := b.pool.Get().(*gowebdav.Client)
		if !ok {
			return errors.Errorf("unexpected webdav client '%T'", client)
		}
		defer b.pool.Put(client)

		if err := fn(client); err != nil {
			return errors.WithStack(err)
		}

		return nil
	}

	fs := NewFs(withClient)

	if err := fn(ctx, fs); err != nil {
		return errors.WithStack(err)
	}

	return nil
}

func New(url string, config *Config) *Backend {
	b := &Backend{
		url:    url,
		config: config,
	}

	b.pool.New = func() any {
		authorizer := gowebdav.NewAutoAuth(b.config.Username, b.config.Password)
		client := gowebdav.NewAuthClient(b.url, authorizer)

		client.SetTimeout(b.config.Timeout)
		client.SetTransport(http.DefaultClient.Transport)

		return client
	}

	return b
}

var _ filesystem.Backend = &Backend{}
