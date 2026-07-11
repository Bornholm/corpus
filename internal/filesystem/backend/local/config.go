package local

import (
	"encoding/json"

	"github.com/bornholm/corpus/internal/filesystem"
	"github.com/bornholm/corpus/internal/filesystem/backend"
	"github.com/pkg/errors"
)

func init() {
	backend.RegisterBackendConfig("local", &Config{}, FromConfig)
}

// Config holds the configuration for a local filesystem backend.
type Config struct {
	Path string `json:"path" jsonschema:"required,description=Absolute path to the base directory"`
}

func FromConfig(configJSON []byte) (filesystem.Backend, error) {
	var cfg Config
	if err := json.Unmarshal(configJSON, &cfg); err != nil {
		return nil, errors.Wrap(err, "could not parse local backend config")
	}
	if cfg.Path == "" {
		return nil, errors.New("local backend config: path is required")
	}
	return New(cfg.Path), nil
}
