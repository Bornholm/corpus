package setup

import (
	"context"

	"github.com/bornholm/corpus/internal/adapter/pipeline"
	"github.com/bornholm/corpus/internal/config"
	"github.com/bornholm/corpus/internal/core/port"
	"github.com/pkg/errors"
)

var getIndexFromConfig = createFromConfigOnce(func(ctx context.Context, conf *config.Config) (port.Index, error) {
	bleveIndex, err := getBleveIndexFromConfig(ctx, conf)
	if err != nil {
		return nil, errors.Wrap(err, "could not create bleve index from config")
	}

	sqlitevecIndex, err := NewSQLiteVecIndexFromConfig(ctx, conf)
	if err != nil {
		return nil, errors.Wrap(err, "could not create sqlitevec index from config")
	}

	llmClient, err := getLLMClientFromConfig(ctx, conf)
	if err != nil {
		return nil, errors.Wrap(err, "could not get llm client from config")
	}

	store, err := getStoreFromConfig(ctx, conf)
	if err != nil {
		return nil, errors.Wrap(err, "could not get store from config")
	}

	weightedIndexes := pipeline.WeightedIndexes{
		pipeline.NewIdentifiedIndex("bleve", bleveIndex):         0.4,
		pipeline.NewIdentifiedIndex("sqlitevec", sqlitevecIndex): 0.6,
	}

	pipelinedIndex := pipeline.NewIndex(
		weightedIndexes,
		pipeline.WithQueryTransformers(
			pipeline.NewHyDEQueryTransformer(llmClient, store),
		),
		pipeline.WithResultsTransformers(
			pipeline.NewDuplicateContentResultsTransformer(store),
			pipeline.NewJudgeResultsTransformer(llmClient, store, conf.LLM.Index.MaxWords),
		),
	)

	return pipelinedIndex, nil
})
