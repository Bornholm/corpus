package webdav

import (
	"net/url"
	"time"

	"github.com/bornholm/corpus/internal/filesystem"
	"github.com/bornholm/corpus/internal/filesystem/backend"
	"github.com/pkg/errors"
)

func init() {
	backend.RegisterBackendFactory("webdav", FromDSN)
}

func FromDSN(dsn *url.URL) (filesystem.Backend, error) {
	scheme := "http://"
	if dsn.Query().Has("useTLS") {
		scheme = "https://"
	}

	url := scheme + dsn.Host + dsn.Path

	config := &Config{
		Timeout: 30 * time.Second,
	}
	configurations := []ConfigureFunc{
		configureCredentials,
		configureTimeout,
	}

	for _, configure := range configurations {
		if err := configure(dsn, config); err != nil {
			return nil, errors.WithStack(err)
		}
	}

	backend := New(url, config)

	return backend, nil
}

type ConfigureFunc func(dsn *url.URL, conf *Config) error

func configureCredentials(dsn *url.URL, config *Config) error {
	config.Username = dsn.User.Username()

	if password, exists := dsn.User.Password(); exists {
		config.Password = password
	}

	return nil
}

const paramTimeout = "timeout"

func configureTimeout(dsn *url.URL, conf *Config) error {
	query := dsn.Query()

	if !query.Has(paramTimeout) {
		return nil
	}

	rawTimeout := query.Get(paramTimeout)
	timeout, err := time.ParseDuration(rawTimeout)
	if err != nil {
		return errors.Wrapf(err, "could not parse query value '%s' as duration", rawTimeout)
	}

	conf.Timeout = timeout

	return nil
}
