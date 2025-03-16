package setup

import (
	"context"

	"github.com/bornholm/corpus/internal/adapter/meta"
	"github.com/bornholm/corpus/internal/config"
	"github.com/bornholm/corpus/internal/core/port"
	"github.com/pkg/errors"
)

var NewIndexFromConfig = createFromConfigOnce(func(ctx context.Context, conf *config.Config) (port.Index, error) {
	bleveIndex, err := NewBleveIndexFromConfig(ctx, conf)
	if err != nil {
		return nil, errors.Wrap(err, "could not create bleve index from config")
	}

	sqlitevecIndex, err := NewSQLiteVecIndexFromConfig(ctx, conf)
	if err != nil {
		return nil, errors.Wrap(err, "could not create sqlitevec index from config")
	}

	metaIndex := meta.NewIndex(meta.WeightedIndexes{
		bleveIndex:     0.4,
		sqlitevecIndex: 0.6,
	})

	return metaIndex, nil
})
