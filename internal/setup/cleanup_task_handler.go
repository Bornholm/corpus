package setup

import (
	"context"

	"github.com/bornholm/corpus/internal/config"
	documentTask "github.com/bornholm/corpus/internal/task/document"
	"github.com/pkg/errors"
)

var getCleanupTaskHandler = createFromConfigOnce(func(ctx context.Context, conf *config.Config) (*documentTask.CleanupHandler, error) {
	documentStore, err := getDocumentStoreFromConfig(ctx, conf)
	if err != nil {
		return nil, errors.Wrap(err, "could not create document store from config")
	}

	index, err := getIndexFromConfig(ctx, conf)
	if err != nil {
		return nil, errors.Wrap(err, "could not create index from config")
	}

	handler := documentTask.NewCleanupHandler(index, documentStore)

	return handler, nil
})
