package setup

import (
	"context"

	gormAdapter "github.com/bornholm/corpus/internal/adapter/gorm"
	"github.com/bornholm/corpus/internal/config"
	"github.com/bornholm/corpus/internal/core/port"
	"github.com/pkg/errors"

	_ "github.com/asg017/sqlite-vec-go-bindings/ncruces"
	"github.com/ncruces/go-sqlite3/gormlite"
	"gorm.io/gorm"
)

var getStoreFromConfig = createFromConfigOnce(func(ctx context.Context, conf *config.Config) (port.Store, error) {
	dialector := gormlite.Open(conf.Storage.Database.DSN)

	db, err := gorm.Open(dialector, &gorm.Config{})
	if err != nil {
		return nil, errors.WithStack(err)
	}

	internalDB, err := db.DB()
	if err != nil {
		return nil, errors.WithStack(err)
	}

	internalDB.SetMaxOpenConns(1)

	if err := db.Exec("PRAGMA journal_mode=wal; PRAGMA foreign_keys=on; PRAGMA busy_timeout=30000").Error; err != nil {
		return nil, errors.WithStack(err)
	}

	return gormAdapter.NewStore(db), nil
})
