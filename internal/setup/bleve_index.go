package setup

import (
	"context"
	"os"

	"github.com/blevesearch/bleve/v2"
	bleveAdapter "github.com/bornholm/corpus/internal/adapter/bleve"
	"github.com/bornholm/corpus/internal/config"
	"github.com/bornholm/corpus/internal/core/port"
	"github.com/pkg/errors"
)

func getBleveIndexFromConfig(ctx context.Context, conf *config.Config) (port.Index, error) {
	var (
		index bleve.Index
		err   error
	)

	stat, err := os.Stat(conf.Storage.Index.DSN)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, errors.WithStack(err)
	}

	if stat == nil {
		mapping := bleveAdapter.IndexMapping()

		index, err = bleve.New(conf.Storage.Index.DSN, mapping)
		if err != nil {
			return nil, errors.WithStack(err)
		}
	} else {
		index, err = bleve.Open(conf.Storage.Index.DSN)
		if err != nil {
			return nil, errors.WithStack(err)
		}
	}

	return bleveAdapter.NewIndex(index), nil
}
