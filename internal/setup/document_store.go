package setup

import (
	"context"

	gormAdapter "github.com/bornholm/corpus/internal/adapter/gorm"
	"github.com/bornholm/corpus/internal/config"
	"github.com/bornholm/corpus/internal/core/port"
	"github.com/pkg/errors"

	_ "github.com/asg017/sqlite-vec-go-bindings/ncruces"
)

var getDocumentStoreFromConfig = createFromConfigOnce(func(ctx context.Context, conf *config.Config) (port.DocumentStore, error) {
	db, err := getGormDatabaseFromConfig(ctx, conf)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	return gormAdapter.NewDocumentStore(db), nil
})
