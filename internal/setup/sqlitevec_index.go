package setup

import (
	"context"

	"github.com/bornholm/corpus/internal/adapter/sqlitevec"
	"github.com/bornholm/corpus/internal/config"
	"github.com/bornholm/corpus/internal/core/port"
	"github.com/ncruces/go-sqlite3"
	"github.com/pkg/errors"
)

func NewSQLiteVecIndexFromConfig(ctx context.Context, conf *config.Config) (port.Index, error) {

	llm, err := getLLMClientFromConfig(ctx, conf)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	db, err := sqlite3.Open(conf.Storage.Database.DSN)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	if err := db.Exec("PRAGMA journal_mode=wal; PRAGMA foreign_keys=on; PRAGMA busy_timeout=30000"); err != nil {
		return nil, errors.WithStack(err)
	}

	return sqlitevec.NewIndex(db, llm, conf.LLM.Index.MaxWords), nil
}
