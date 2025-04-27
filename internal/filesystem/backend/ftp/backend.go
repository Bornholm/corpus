package ftp

import (
	"context"
	"log/slog"

	"github.com/bornholm/corpus/internal/filesystem"
	"github.com/jlaffaye/ftp"
	"github.com/pkg/errors"
	"github.com/spf13/afero"
)

type Backend struct {
	addr     string
	basePath string
	username string
	password string
	options  []ftp.DialOption
}

// Mount implements fs.Backend.
func (b *Backend) Mount(ctx context.Context, fn func(ctx context.Context, fs afero.Fs) error) error {
	withConn := func(fn func(*ftp.ServerConn) error) error {
		options := append([]ftp.DialOption{
			ftp.DialWithContext(ctx),
		}, b.options...)

		conn, err := ftp.Dial(b.addr, options...)
		if err != nil {
			return errors.WithStack(err)
		}

		defer func() {
			if err := conn.Quit(); err != nil {
				err = errors.WithStack(err)
				slog.ErrorContext(ctx, "could not quit ftp server", slog.Any("error", err))
			}
		}()

		if b.username != "" && b.password != "" {
			if err := conn.Login(b.username, b.password); err != nil {
				return errors.WithStack(err)
			}

			defer func() {
				if err := conn.Logout(); err != nil && !isNotImplementedErr(err) && !isBadCommand(err) {
					err = errors.WithStack(err)
					slog.ErrorContext(ctx, "could not logout from ftp server", slog.Any("error", err))
				}
			}()
		}

		if err := fn(conn); err != nil {
			return errors.WithStack(err)
		}

		return nil
	}

	ftpFs := NewFs(withConn)

	var fs afero.Fs = ftpFs
	if b.basePath != "" {
		fs = afero.NewBasePathFs(ftpFs, b.basePath)
	}

	if err := fn(ctx, fs); err != nil {
		return errors.WithStack(err)
	}

	return nil
}

func New(addr string, basePath string, username, password string, options ...ftp.DialOption) *Backend {
	return &Backend{
		addr:     addr,
		basePath: basePath,
		username: username,
		password: password,
		options:  options,
	}
}

var _ filesystem.Backend = &Backend{}
