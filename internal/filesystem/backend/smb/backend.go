package smb

import (
	"context"
	"net"

	"github.com/bornholm/corpus/internal/filesystem"
	"github.com/getsentry/sentry-go"
	"github.com/hirochachacha/go-smb2"
	"github.com/pkg/errors"
	"github.com/spf13/afero"
	"gitlab.com/wpetit/goweb/logger"
)

type Backend struct {
	addr     string
	basePath string
	config   *Config
}

type Config struct {
	Initiator smb2.Initiator
	ShareName string
}

// Mount implements fs.Backend.
func (b *Backend) Mount(ctx context.Context, fn func(ctx context.Context, fs afero.Fs) error) error {
	conn, err := net.Dial("tcp", b.addr)
	if err != nil {
		return errors.WithStack(err)
	}

	defer func() {
		if err := conn.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
			err = errors.WithStack(err)
			sentry.CaptureException(err)
			logger.Error(ctx, "could not close ssh connection", logger.E(err))
		}
	}()

	dialer := &smb2.Dialer{
		Initiator: b.config.Initiator,
	}

	session, err := dialer.Dial(conn)
	if err != nil {
		return errors.WithStack(err)
	}

	session = session.WithContext(ctx)

	defer func() {
		if err := session.Logoff(); err != nil {
			var contextErr *smb2.ContextError
			if errors.As(err, &contextErr) {
				return
			}

			err = errors.WithStack(err)
			sentry.CaptureException(err)
			logger.Error(ctx, "could not logout samba session", logger.E(err))
		}
	}()

	share, err := session.Mount(b.config.ShareName)
	if err != nil {
		return errors.WithStack(err)
	}

	defer func() {
		if err := share.Umount(); err != nil {
			err = errors.WithStack(err)
			sentry.CaptureException(err)
			logger.Error(ctx, "could not unmount samba share", logger.E(err))
		}
	}()

	var fs afero.Fs
	if b.basePath != "" {
		fs = afero.NewBasePathFs(NewFs(share), b.basePath)
	} else {
		fs = NewFs(share)
	}

	if err := fn(ctx, fs); err != nil {
		return errors.WithStack(err)
	}

	return nil
}

func New(addr string, basePath string, config *Config) *Backend {
	return &Backend{
		addr:     addr,
		basePath: basePath,
		config:   config,
	}
}

var _ filesystem.Backend = &Backend{}
