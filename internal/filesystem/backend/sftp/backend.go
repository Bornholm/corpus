package sftp

import (
	"context"
	"net"

	"github.com/bornholm/corpus/internal/filesystem"
	"github.com/getsentry/sentry-go"
	"github.com/pkg/errors"
	"github.com/pkg/sftp"
	"github.com/spf13/afero"
	"github.com/spf13/afero/sftpfs"
	"gitlab.com/wpetit/goweb/logger"
	"golang.org/x/crypto/ssh"
)

type Backend struct {
	addr     string
	basePath string
	config   *ssh.ClientConfig
}

// Mount implements fs.Backend.
func (b *Backend) Mount(ctx context.Context, fn func(ctx context.Context, fs afero.Fs) error) error {
	sshClient, err := ssh.Dial("tcp", b.addr, b.config)
	if err != nil {
		return errors.WithStack(err)
	}

	defer func() {
		if err := sshClient.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
			err = errors.WithStack(err)
			sentry.CaptureException(err)
			logger.Error(ctx, "could not close ssh connection", logger.E(err))
		}
	}()

	sftpClient, err := sftp.NewClient(sshClient)
	if err != nil {
		return errors.WithStack(err)
	}

	defer func() {
		if err := sftpClient.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
			err = errors.WithStack(err)
			sentry.CaptureException(err)
			logger.Error(ctx, "could not close sftp connection", logger.E(err))
		}
	}()

	fs := sftpfs.New(sftpClient)

	if b.basePath != "" {
		fs = afero.NewBasePathFs(fs, b.basePath)
	}

	if err := fn(ctx, fs); err != nil {
		return errors.WithStack(err)
	}

	return nil
}

func New(addr string, basePath string, config *ssh.ClientConfig) *Backend {
	return &Backend{
		addr:     addr,
		config:   config,
		basePath: basePath,
	}
}

var _ filesystem.Backend = &Backend{}
