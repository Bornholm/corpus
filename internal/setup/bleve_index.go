package setup

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/blevesearch/bleve/v2"
	"github.com/blevesearch/bleve/v2/mapping"
	bleveAdapter "github.com/bornholm/corpus/internal/adapter/bleve"
	"github.com/bornholm/corpus/internal/config"
	"github.com/bornholm/corpus/internal/core/port"
	"github.com/pkg/errors"
)

const mappingHashFilename = ".mapping_hash"

// bleveReindexNeeded is set to true when a mapping change is detected at startup.
// The actual scheduling is deferred to setupTaskHandlers, after handlers are registered.
var bleveReindexNeeded bool

// cachedBleveIndex is a cached reference to the bleve index to avoid
// re-opening the index in the reindex task handler (which can cause deadlocks).
var cachedBleveIndex port.Index

// reindexBleveIndex is an alias for cachedBleveIndex for use by the reindex task handler.
// This is needed because the reindex task handler is in a different package (setup) but
// needs to access the same cached index instance.
var reindexBleveIndex port.Index

func mappingHash(mapping *mapping.IndexMappingImpl) (string, error) {
	data, err := json.Marshal(mapping)
	if err != nil {
		return "", errors.Wrap(err, "could not marshal mapping")
	}
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:]), nil
}

func readStoredMappingHash(indexPath string) (string, error) {
	hashFile := filepath.Join(indexPath, mappingHashFilename)
	data, err := os.ReadFile(hashFile)
	if err != nil {
		return "", errors.WithStack(err)
	}
	return string(data), nil
}

func writeStoredMappingHash(indexPath, hash string) error {
	hashFile := filepath.Join(indexPath, mappingHashFilename)
	if err := os.WriteFile(hashFile, []byte(hash), 0600); err != nil {
		return errors.Wrap(err, "could not write mapping hash")
	}
	return nil
}

func getBleveIndexFromConfig(ctx context.Context, conf *config.Config) (port.Index, error) {
	var (
		index bleve.Index
		err   error
	)

	stat, err := os.Stat(conf.Storage.Bleve.DSN)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, errors.WithStack(err)
	}

	indexPath := conf.Storage.Bleve.DSN

	if stat == nil {
		mapping := bleveAdapter.IndexMapping()

		index, err = bleve.New(indexPath, mapping)
		if err != nil {
			return nil, errors.Wrap(err, "could not create bleve index")
		}

		// Store the mapping hash for future comparisons
		hash, err := mappingHash(mapping)
		if err != nil {
			slog.WarnContext(ctx, "could not compute mapping hash", slog.Any("error", err))
		} else {
			if err := writeStoredMappingHash(indexPath, hash); err != nil {
				slog.WarnContext(ctx, "could not store mapping hash", slog.Any("error", err))
			}
		}
	} else {
		// Check if the mapping has changed
		currentMapping := bleveAdapter.IndexMapping()
		currentHash, err := mappingHash(currentMapping)
		if err != nil {
			slog.WarnContext(ctx, "could not compute current mapping hash", slog.Any("error", err))
		} else {
			var storedHash string
			storedHash, err = readStoredMappingHash(indexPath)
			if err != nil {
				slog.WarnContext(ctx, "could not read stored mapping hash, index may have been created with an older version", slog.Any("error", err))
			}

			if storedHash != currentHash {
				slog.InfoContext(ctx, "bleve index mapping has changed, deleting old index and creating fresh one",
					slog.String("stored_hash", storedHash),
					slog.String("current_hash", currentHash))

				// Delete the old index files to get a fresh, compact index
				slog.InfoContext(ctx, "deleting old bleve index files", slog.String("path", indexPath))
				if err := os.RemoveAll(indexPath); err != nil {
					slog.WarnContext(ctx, "could not delete old bleve index, will try to reindex in place", slog.Any("error", err))
					// Fall back to old behavior: open existing index
					index, err = bleve.Open(indexPath)
					if err != nil {
						return nil, errors.Wrap(err, "could not open old bleve index for reindex")
					}
				} else {
					// Create a fresh new index
					currentMapping = bleveAdapter.IndexMapping()
					index, err = bleve.New(indexPath, currentMapping)
					if err != nil {
						return nil, errors.Wrap(err, "could not create new bleve index")
					}
				}

				// Store the new hash
				if err := writeStoredMappingHash(indexPath, currentHash); err != nil {
					slog.WarnContext(ctx, "could not store mapping hash after rebuild", slog.Any("error", err))
				}

				// Signal that a reindex is needed. The task will be scheduled in
				// setupTaskHandlers, after all handlers are registered, to avoid a
				// race where the task fires before its handler exists.
				bleveReindexNeeded = true

				// Cache the index for use by the reindex task handler
				cachedBleveIndex = bleveAdapter.NewIndex(index)
				reindexBleveIndex = cachedBleveIndex

				return cachedBleveIndex, nil
			}
		}

		// Open existing index
		index, err = bleve.Open(indexPath)
		if err != nil {
			return nil, errors.Wrap(err, "could not open bleve index")
		}
	}

	// Cache the index for use by the reindex task handler
	cachedBleveIndex = bleveAdapter.NewIndex(index)
	reindexBleveIndex = cachedBleveIndex

	return cachedBleveIndex, nil
}

// IndexMappingHash returns the hash of the current index mapping.
// This can be used to verify the mapping version.
func IndexMappingHash() (string, error) {
	mapping := bleveAdapter.IndexMapping()
	return mappingHash(mapping)
}
