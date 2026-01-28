package setup

import (
	"context"

	"github.com/bornholm/corpus/internal/config"
	"github.com/bornholm/corpus/internal/task/index"
	indexTask "github.com/bornholm/corpus/internal/task/index"
	"github.com/pkg/errors"
)

var getIndexFileTaskHandler = createFromConfigOnce(func(ctx context.Context, conf *config.Config) (*index.IndexFileHandler, error) {
	documentStore, err := getDocumentStoreFromConfig(ctx, conf)
	if err != nil {
		return nil, errors.Wrap(err, "could not create document store from config")
	}

	userStore, err := getUserStoreFromConfig(ctx, conf)
	if err != nil {
		return nil, errors.Wrap(err, "could not create document store from config")
	}

	fileConverter, err := getFileConverterFromConfig(ctx, conf)
	if err != nil {
		return nil, errors.Wrap(err, "could not create file converter from config")
	}

	index, err := getIndexFromConfig(ctx, conf)
	if err != nil {
		return nil, errors.Wrap(err, "could not create index from config")
	}

	handler := indexTask.NewIndexFileHandler(userStore, documentStore, fileConverter, index, conf.LLM.Index.MaxWords)

	return handler, nil
})
