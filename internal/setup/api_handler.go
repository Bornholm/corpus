package setup

import (
	"context"

	"github.com/bornholm/corpus/internal/config"
	"github.com/bornholm/corpus/internal/http/handler/api"
	"github.com/pkg/errors"
)

func NewAPIHandlerFromConfig(ctx context.Context, conf *config.Config) (*api.Handler, error) {
	store, err := NewStoreFromConfig(ctx, conf)
	if err != nil {
		return nil, errors.Wrap(err, "could not create store from config")
	}

	index, err := NewIndexFromConfig(ctx, conf)
	if err != nil {
		return nil, errors.Wrap(err, "could not create index from config")
	}

	handler := api.NewHandler(store, index)

	return handler, nil
}
