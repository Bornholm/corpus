package setup

import (
	"context"

	"github.com/bornholm/corpus/internal/config"
	"github.com/bornholm/corpus/internal/http/handler/api"
	"github.com/pkg/errors"
)

func getAPIHandlerFromConfig(ctx context.Context, conf *config.Config) (*api.Handler, error) {
	documentManager, err := getDocumentManager(ctx, conf)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	backupManager, err := getBackupManager(ctx, conf)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	taskRunner, err := getTaskRunner(ctx, conf)
	if err != nil {
		return nil, errors.Wrap(err, "could not create task runner from config")
	}

	handler := api.NewHandler(documentManager, backupManager, taskRunner)

	return handler, nil
}
