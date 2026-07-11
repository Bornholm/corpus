package webdav

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/bornholm/corpus/internal/filesystem"
	"github.com/bornholm/corpus/internal/filesystem/backend"
	"github.com/pkg/errors"
)

func init() {
	backend.RegisterBackendConfig("webdav", &WebDAVConfig{}, FromConfig)
}

// WebDAVConfig holds the configuration for a WebDAV backend.
// Named WebDAVConfig to avoid collision with the internal Config type in this package.
type WebDAVConfig struct {
	Host     string `json:"host"               jsonschema:"required,description=WebDAV server hostname or IP"`
	Port     int    `json:"port,omitempty"     jsonschema:"description=WebDAV server port (default: 80 or 443)"`
	Path     string `json:"path,omitempty"     jsonschema:"description=WebDAV resource path"`
	Username string `json:"username,omitempty" jsonschema:"description=WebDAV username"`
	Password string `json:"password,omitempty" jsonschema:"description=WebDAV password"`
	UseTLS   bool   `json:"useTLS,omitempty"   jsonschema:"description=Use HTTPS instead of HTTP"`
	Timeout  string `json:"timeout,omitempty"  jsonschema:"default=30s,description=Connection timeout (e.g. 30s)"`
}

func FromConfig(configJSON []byte) (filesystem.Backend, error) {
	var cfg WebDAVConfig
	if err := json.Unmarshal(configJSON, &cfg); err != nil {
		return nil, errors.Wrap(err, "could not parse webdav backend config")
	}
	if cfg.Host == "" {
		return nil, errors.New("webdav backend config: host is required")
	}

	scheme := "http"
	if cfg.UseTLS {
		scheme = "https"
	}

	var rawURL string
	if cfg.Port != 0 {
		rawURL = fmt.Sprintf("%s://%s:%d%s", scheme, cfg.Host, cfg.Port, cfg.Path)
	} else {
		rawURL = fmt.Sprintf("%s://%s%s", scheme, cfg.Host, cfg.Path)
	}

	internalCfg := &Config{
		Username: cfg.Username,
		Password: cfg.Password,
		Timeout:  30 * time.Second,
	}

	if cfg.Timeout != "" {
		d, err := time.ParseDuration(cfg.Timeout)
		if err != nil {
			return nil, errors.Wrapf(err, "could not parse timeout '%s'", cfg.Timeout)
		}
		internalCfg.Timeout = d
	}

	return New(rawURL, internalCfg), nil
}
