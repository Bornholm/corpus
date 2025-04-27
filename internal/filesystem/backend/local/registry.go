package local

import (
	"net/url"
	"strings"

	"github.com/bornholm/corpus/internal/filesystem"
	"github.com/bornholm/corpus/internal/filesystem/backend"
)

func init() {
	backend.RegisterBackendFactory("local", FromDSN)
}

func FromDSN(dsn *url.URL) (filesystem.Backend, error) {
	basePath := dsn.Host + "/" + strings.TrimPrefix(dsn.Path, "/")
	backend := New(basePath)
	return backend, nil
}
