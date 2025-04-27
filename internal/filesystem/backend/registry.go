package backend

import (
	"net/url"

	"github.com/bornholm/corpus/internal/filesystem"
	"github.com/pkg/errors"
)

var backendFactories = make(map[string]BackendFactory, 0)

type BackendFactory func(url *url.URL) (filesystem.Backend, error)

func RegisterBackendFactory(scheme string, factory BackendFactory) {
	backendFactories[scheme] = factory
}

func New(dsn string) (filesystem.Backend, error) {
	url, err := url.Parse(dsn)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	factory, exists := backendFactories[url.Scheme]
	if !exists {
		return nil, errors.Wrapf(ErrSchemeNotRegistered, "no driver associated with scheme '%s'", url.Scheme)
	}

	store, err := factory(url)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	return store, nil
}
