package setup

import (
	"context"

	"github.com/bornholm/corpus/internal/config"
	"github.com/bornholm/corpus/internal/core/service"
	"github.com/pkg/errors"
)

var getDocumentManager = createFromConfigOnce(func(ctx context.Context, conf *config.Config) (*service.DocumentManager, error) {
	store, err := getDocumentStoreFromConfig(ctx, conf)
	if err != nil {
		return nil, errors.Wrap(err, "could not create store from config")
	}

	index, err := getIndexFromConfig(ctx, conf)
	if err != nil {
		return nil, errors.Wrap(err, "could not create index from config")
	}

	options := []service.DocumentManagerOptionFunc{}

	if conf.FileConverter.Enabled {
		fileConverter, err := getFileConverterFromConfig(ctx, conf)
		if err != nil {
			return nil, errors.WithStack(err)
		}

		options = append(options, service.WithDocumentManagerFileConverter(fileConverter))
	}

	taskRunner, err := getTaskRunner(ctx, conf)
	if err != nil {
		return nil, errors.Wrap(err, "could not create task runner from config")
	}

	llmClient, err := getLLMClientFromConfig(ctx, conf)
	if err != nil {
		return nil, errors.Wrap(err, "could not create llm client from config")
	}

	documentManager := service.NewDocumentManager(store, index, taskRunner, llmClient, options...)

	return documentManager, nil
})
