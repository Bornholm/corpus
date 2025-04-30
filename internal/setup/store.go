package setup

import (
	"context"
	"log/slog"

	gormAdapter "github.com/bornholm/corpus/internal/adapter/gorm"
	"github.com/bornholm/corpus/internal/config"
	"github.com/bornholm/corpus/internal/core/port"
	"github.com/pkg/errors"

	_ "github.com/asg017/sqlite-vec-go-bindings/ncruces"
	"github.com/ncruces/go-sqlite3/gormlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var getStoreFromConfig = createFromConfigOnce(func(ctx context.Context, conf *config.Config) (port.Store, error) {
	dialector := gormlite.Open(conf.Storage.Database.DSN)

	var logLevel logger.LogLevel
	switch slog.Level(conf.Logger.Level) {
	case slog.LevelError:
		logLevel = logger.Error
	case slog.LevelWarn:
		logLevel = logger.Warn
	case slog.LevelInfo:
		logLevel = logger.Info
	default:
		logLevel = logger.Error
	}

	db, err := gorm.Open(dialector, &gorm.Config{
		Logger: logger.Default.LogMode(logLevel),
	})
	if err != nil {
		return nil, errors.WithStack(err)
	}

	if slog.Level(conf.Logger.Level) == slog.LevelDebug {
		db = db.Debug()
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
