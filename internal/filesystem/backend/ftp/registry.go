package ftp

import (
	"net/url"
	"strings"
	"time"

	"github.com/bornholm/corpus/internal/filesystem"
	"github.com/bornholm/corpus/internal/filesystem/backend"
	"github.com/jlaffaye/ftp"
	"github.com/pkg/errors"
)

func init() {
	backend.RegisterBackendFactory("ftp", FromDSN)
}

func FromDSN(dsn *url.URL) (filesystem.Backend, error) {
	addr := dsn.Host
	basePath := strings.TrimPrefix(dsn.Path, "/")

	options := make([]ftp.DialOption, 0)

	configurations := []ConfigureFunc{
		configureTimeout,
	}

	for _, configure := range configurations {
		if err := configure(dsn, &options); err != nil {
			return nil, errors.WithStack(err)
		}
	}

	username := dsn.User.Username()
	password, _ := dsn.User.Password()

	backend := New(addr, basePath, username, password, options...)

	return backend, nil
}

type ConfigureFunc func(dsn *url.URL, options *[]ftp.DialOption) error

const paramTimeout = "timeout"

func configureTimeout(dsn *url.URL, options *[]ftp.DialOption) error {
	query := dsn.Query()

	if !query.Has(paramTimeout) {
		return nil
	}

	rawTimeout := query.Get(paramTimeout)
	timeout, err := time.ParseDuration(rawTimeout)
	if err != nil {
		return errors.Wrapf(err, "could not parse query value '%s' as duration", rawTimeout)
	}

	*options = append(*options, ftp.DialWithTimeout(timeout))

	return nil
}
