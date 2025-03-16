package setup

import (
	"context"

	gormAdapter "github.com/bornholm/corpus/internal/adapter/gorm"
	"github.com/bornholm/corpus/internal/config"
	"github.com/bornholm/corpus/internal/core/port"
	"github.com/pkg/errors"

	_ "embed"

	_ "github.com/asg017/sqlite-vec-go-bindings/ncruces"
	"github.com/ncruces/go-sqlite3/gormlite"
	"gorm.io/gorm"
)

var NewStoreFromConfig = createFromConfigOnce(func(ctx context.Context, conf *config.Config) (port.Store, error) {
	dialector := gormlite.Open(conf.Storage.Database.DSN)

	db, err := gorm.Open(dialector, &gorm.Config{})
	if err != nil {
		return nil, errors.WithStack(err)
	}

	return gormAdapter.NewStore(db), nil
})
