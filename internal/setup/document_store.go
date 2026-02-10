package setup

import (
	"context"
	"log/slog"

	"github.com/bornholm/corpus/internal/adapter/cache"
	"github.com/bornholm/corpus/internal/config"
	"github.com/bornholm/corpus/internal/core/port"
	"github.com/pkg/errors"

	_ "github.com/asg017/sqlite-vec-go-bindings/ncruces"
)

var getDocumentStoreFromConfig = createFromConfigOnce(func(ctx context.Context, conf *config.Config) (port.DocumentStore, error) {
	var (
		store port.DocumentStore
		err   error
	)

	store, err = getGormStoreFromConfig(ctx, conf)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	if conf.Storage.Database.Cache.Documents.Enabled {
		slog.DebugContext(ctx, "using cached document store", slog.Duration("ttl", conf.Storage.Database.Cache.Documents.TTL), slog.Int("cache_size", conf.Storage.Database.Cache.Documents.Size))
		store = cache.NewDocumentStore(store, conf.Storage.Database.Cache.Documents.Size, conf.Storage.Database.Cache.Documents.TTL)
	}

	return store, nil
})
