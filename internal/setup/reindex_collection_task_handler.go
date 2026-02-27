package setup

import (
	"context"

	"github.com/bornholm/corpus/internal/config"
	documentTask "github.com/bornholm/corpus/internal/task/document"
	"github.com/pkg/errors"
)

var getReindexCollectionTaskHandler = createFromConfigOnce(func(ctx context.Context, conf *config.Config) (*documentTask.ReindexCollectionHandler, error) {
	documentStore, err := getDocumentStoreFromConfig(ctx, conf)
	if err != nil {
		return nil, errors.Wrap(err, "could not create document store from config")
	}

	index, err := getIndexFromConfig(ctx, conf)
	if err != nil {
		return nil, errors.Wrap(err, "could not create index from config")
	}

	handler := documentTask.NewReindexCollectionHandler(documentStore, index, conf.LLM.Index.MaxWords)

	return handler, nil
})
