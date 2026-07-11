package git

import (
	"encoding/json"
	"time"

	"github.com/bornholm/corpus/internal/filesystem"
	"github.com/bornholm/corpus/internal/filesystem/backend"
	"github.com/pkg/errors"
)

func init() {
	backend.RegisterBackendConfig("git", &GitConfig{}, FromConfig)
}

// GitConfig holds the configuration for a Git repository backend.
// Named GitConfig to avoid collision with the internal Config type in this package.
type GitConfig struct {
	URL          string `json:"url"                  jsonschema:"required,description=Git repository URL (https:// or http://)"`
	Branch       string `json:"branch,omitempty"     jsonschema:"description=Branch to clone (defaults to HEAD)"`
	PullInterval string `json:"pullInterval,omitempty" jsonschema:"default=30m,description=Interval between auto-pull operations (e.g. 30m)"`
}

func FromConfig(configJSON []byte) (filesystem.Backend, error) {
	var cfg GitConfig
	if err := json.Unmarshal(configJSON, &cfg); err != nil {
		return nil, errors.Wrap(err, "could not parse git backend config")
	}
	if cfg.URL == "" {
		return nil, errors.New("git backend config: url is required")
	}

	pullInterval := 30 * time.Minute
	if cfg.PullInterval != "" {
		d, err := time.ParseDuration(cfg.PullInterval)
		if err != nil {
			return nil, errors.Wrapf(err, "could not parse pullInterval '%s'", cfg.PullInterval)
		}
		pullInterval = d
	}

	return New(cfg.URL, cfg.Branch, pullInterval), nil
}
