package ftp

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/bornholm/corpus/internal/filesystem"
	"github.com/bornholm/corpus/internal/filesystem/backend"
	goftp "github.com/jlaffaye/ftp"
	"github.com/pkg/errors"
)

func init() {
	backend.RegisterBackendConfig("ftp", &Config{}, FromConfig)
}

// Config holds the configuration for an FTP backend.
type Config struct {
	Host     string `json:"host"               jsonschema:"required,description=FTP server hostname or IP"`
	Port     int    `json:"port,omitempty"     jsonschema:"default=21,description=FTP server port"`
	Username string `json:"username,omitempty" jsonschema:"description=FTP username"`
	Password string `json:"password,omitempty" jsonschema:"description=FTP password"`
	BasePath string `json:"basePath,omitempty" jsonschema:"description=Base path on the FTP server"`
	Timeout  string `json:"timeout,omitempty"  jsonschema:"default=30s,description=Connection timeout (e.g. 30s)"`
}

func FromConfig(configJSON []byte) (filesystem.Backend, error) {
	var cfg Config
	if err := json.Unmarshal(configJSON, &cfg); err != nil {
		return nil, errors.Wrap(err, "could not parse ftp backend config")
	}
	if cfg.Host == "" {
		return nil, errors.New("ftp backend config: host is required")
	}

	port := cfg.Port
	if port == 0 {
		port = 21
	}

	addr := fmt.Sprintf("%s:%d", cfg.Host, port)

	opts := make([]goftp.DialOption, 0)
	if cfg.Timeout != "" {
		d, err := time.ParseDuration(cfg.Timeout)
		if err != nil {
			return nil, errors.Wrapf(err, "could not parse timeout '%s'", cfg.Timeout)
		}
		opts = append(opts, goftp.DialWithTimeout(d))
	}

	return New(addr, cfg.BasePath, cfg.Username, cfg.Password, opts...), nil
}
