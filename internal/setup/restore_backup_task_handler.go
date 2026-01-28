package setup

import (
	"context"

	"github.com/bornholm/corpus/internal/config"
	"github.com/bornholm/corpus/internal/core/service/backup"
	"github.com/pkg/errors"
)

var getRestoreBackupTaskHandler = createFromConfigOnce(func(ctx context.Context, conf *config.Config) (*backup.RestoreBackupHandler, error) {
	documentStore, err := getDocumentStoreFromConfig(ctx, conf)
	if err != nil {
		return nil, errors.Wrap(err, "could not create document store from config")
	}

	backupManager, err := getBackupManager(ctx, conf)
	if err != nil {
		return nil, errors.Wrap(err, "could not create backup manager from config")
	}

	handler := backup.NewRestoreBackupHandler(documentStore, backupManager)

	return handler, nil
})
