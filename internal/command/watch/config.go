package watch

import (
	"context"
	"encoding/json"
	"io"
	"net/url"
	"path/filepath"
	"regexp"
	"time"

	"github.com/Bornholm/amatl/pkg/resolver"
	"github.com/bornholm/corpus/internal/filesystem"
	fsbackend "github.com/bornholm/corpus/internal/filesystem/backend"
	"github.com/bornholm/corpus/pkg/model"
	"github.com/pkg/errors"
	"github.com/redmatter/go-globre/v2"
	"gopkg.in/yaml.v2"
)

// WatchConfigFile represents the top-level YAML config for the watch command.
type WatchConfigFile struct {
	Sources []WatchSource `yaml:"sources"`
}

// WatchSource describes one filesystem source to watch.
type WatchSource struct {
	Label       string             `yaml:"label"`
	Backend     map[string]any     `yaml:"backend"`
	Collections []string           `yaml:"collections"`
	Options     WatchSourceOptions `yaml:"options"`
}

// WatchSourceOptions holds the per-source watcher and indexer options.
type WatchSourceOptions struct {
	Filter         string `yaml:"filter"`
	ETagStrategy   string `yaml:"etagStrategy"` // "modtime" (default) or "size"
	Recursive      *bool  `yaml:"recursive"`
	Directory      string `yaml:"directory"`
	Concurrency    int    `yaml:"concurrency"`
	DeleteOrphans  bool   `yaml:"deleteOrphans"`
	SyncOnStart    *bool  `yaml:"syncOnStart"`
	WatchInterval  string `yaml:"watchInterval"`
	SourceTemplate string `yaml:"source"` // URL template with __PATH__, or "embedded"
}

// resolvedWatchSource holds everything needed to start a filesystemIndexer.
type resolvedWatchSource struct {
	label          string
	backend        filesystem.Backend
	collections    []model.CollectionID
	source         *url.URL
	sourceEmbedded bool
	eTagType       ETagType
	iopts          indexerOpts
	watchOptions   []filesystem.WatchOptionFunc
}

// loadWatchConfig reads the config file and returns the sources list.
func loadWatchConfig(ctx context.Context, configURL string) (*WatchConfigFile, error) {
	u, err := url.Parse(configURL)
	if err != nil {
		return nil, errors.Wrapf(err, "could not parse config url '%s'", configURL)
	}

	reader, err := resolver.Resolve(ctx, u)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	defer reader.Close()

	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	var cfg WatchConfigFile
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, errors.WithStack(err)
	}

	// Resolve relative file paths in FileRef values inside backend maps
	configDir := filepath.Dir(u.Path)
	for i := range cfg.Sources {
		resolveFileRefs(cfg.Sources[i].Backend, configDir)
	}

	return &cfg, nil
}

// resolveFileRefs walks a backend config map and resolves relative `path:` values in FileRef objects.
func resolveFileRefs(m map[string]any, configDir string) {
	for key, val := range m {
		if sub, ok := val.(map[string]any); ok {
			// Check if it looks like a FileRef (has "path" key with a relative path)
			if pathVal, ok := sub["path"].(string); ok && pathVal != "" && !filepath.IsAbs(pathVal) {
				sub["path"] = filepath.Join(configDir, pathVal)
				m[key] = sub
			}
		}
	}
}

// resolveWatchSource builds the concrete indexer config from a WatchSource.
func resolveWatchSource(src WatchSource) (*resolvedWatchSource, error) {
	// --- Backend ---
	backendTypeRaw, ok := src.Backend["type"]
	if !ok {
		return nil, errors.Errorf("source '%s': backend config must have a 'type' field", src.Label)
	}
	backendType, ok := backendTypeRaw.(string)
	if !ok || backendType == "" {
		return nil, errors.Errorf("source '%s': backend 'type' must be a non-empty string", src.Label)
	}

	// Re-marshal the backend map to JSON, excluding the 'type' discriminator
	configMap := make(map[string]any, len(src.Backend))
	for k, v := range src.Backend {
		if k != "type" {
			configMap[k] = v
		}
	}
	configJSON, err := json.Marshal(configMap)
	if err != nil {
		return nil, errors.Wrapf(err, "source '%s': could not marshal backend config", src.Label)
	}

	b, err := fsbackend.NewFromConfig(backendType, configJSON)
	if err != nil {
		return nil, errors.Wrapf(err, "source '%s': could not create backend of type '%s'", src.Label, backendType)
	}

	// --- Collections ---
	collIDs := make([]model.CollectionID, len(src.Collections))
	for i, c := range src.Collections {
		collIDs[i] = model.CollectionID(c)
	}

	// --- ETag type ---
	eTagType := ETagTypeModTime
	if src.Options.ETagStrategy == string(ETagTypeSize) {
		eTagType = ETagTypeSize
	}

	// --- Source URL / embedded ---
	var sourceURL *url.URL
	var sourceEmbedded bool
	if tmpl := src.Options.SourceTemplate; tmpl != "" {
		if tmpl == "embedded" {
			sourceEmbedded = true
		} else {
			parsed, err := url.Parse(tmpl)
			if err != nil {
				return nil, errors.Wrapf(err, "source '%s': could not parse source template '%s'", src.Label, tmpl)
			}
			sourceURL = parsed
		}
	}

	// --- Indexer options ---
	iopts := indexerOpts{
		concurrency:   8,
		syncOnStart:   true,
		deleteOrphans: false,
	}
	o := src.Options
	if o.Concurrency > 0 {
		iopts.concurrency = o.Concurrency
	}
	if o.SyncOnStart != nil {
		iopts.syncOnStart = *o.SyncOnStart
	}
	if o.DeleteOrphans {
		iopts.deleteOrphans = true
	}

	// --- Watch options ---
	var watchOptions []filesystem.WatchOptionFunc

	recursive := true
	if o.Recursive != nil {
		recursive = *o.Recursive
	}
	watchOptions = append(watchOptions, filesystem.WithRecursive(recursive))

	if o.WatchInterval != "" {
		interval, err := time.ParseDuration(o.WatchInterval)
		if err != nil {
			return nil, errors.Wrapf(err, "source '%s': could not parse watchInterval '%s'", src.Label, o.WatchInterval)
		}
		watchOptions = append(watchOptions, filesystem.WithInterval(interval))
	}

	if o.Directory != "" {
		watchOptions = append(watchOptions, filesystem.WithDirectory(o.Directory))
	}

	if o.Filter != "" {
		pathRegExp := globre.RegexFromGlob(
			o.Filter,
			globre.ExtendedSyntaxEnabled(true),
			globre.GlobStarEnabled(true),
			globre.WithDelimiter('/'),
		)
		filter, err := regexp.Compile(pathRegExp)
		if err != nil {
			return nil, errors.Wrapf(err, "source '%s': could not compile filter '%s'", src.Label, o.Filter)
		}
		watchOptions = append(watchOptions, filesystem.WithFilter(filter))
	}

	return &resolvedWatchSource{
		label:          src.Label,
		backend:        b,
		collections:    collIDs,
		source:         sourceURL,
		sourceEmbedded: sourceEmbedded,
		eTagType:       eTagType,
		iopts:          iopts,
		watchOptions:   watchOptions,
	}, nil
}
