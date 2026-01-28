package setup

import (
	"context"

	"github.com/bornholm/corpus/internal/config"
	"github.com/bornholm/corpus/internal/task/index"
	indexTask "github.com/bornholm/corpus/internal/task/index"
	"github.com/pkg/errors"
)

var getCleanupIndexTaskHandler = createFromConfigOnce(func(ctx context.Context, conf *config.Config) (*index.CleanupIndexHandler, error) {
	documentStore, err := getDocumentStoreFromConfig(ctx, conf)
	if err != nil {
		return nil, errors.Wrap(err, "could not create document store from config")
	}

	index, err := getIndexFromConfig(ctx, conf)
	if err != nil {
		return nil, errors.Wrap(err, "could not create index from config")
	}

	handler := indexTask.NewCleanupIndexHandler(index, documentStore)

	return handler, nil
})
