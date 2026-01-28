package setup

import (
	"context"

	"github.com/bornholm/corpus/internal/config"
	"github.com/bornholm/corpus/internal/core/service/backup"
	"github.com/pkg/errors"
)

var getBackupManager = createFromConfigOnce(func(ctx context.Context, conf *config.Config) (*backup.Manager, error) {
	store, err := getDocumentStoreFromConfig(ctx, conf)
	if err != nil {
		return nil, errors.Wrap(err, "could not create store from config")
	}

	index, err := getIndexFromConfig(ctx, conf)
	if err != nil {
		return nil, errors.Wrap(err, "could not create index from config")
	}

	taskRunner, err := getTaskRunner(ctx, conf)
	if err != nil {
		return nil, errors.Wrap(err, "could not create task runner from config")
	}

	backupManager := backup.NewManager(index, store, taskRunner)

	return backupManager, nil
})
