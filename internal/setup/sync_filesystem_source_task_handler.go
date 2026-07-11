package setup

import (
	"context"

	"github.com/bornholm/corpus/internal/config"
	documentTask "github.com/bornholm/corpus/internal/task/document"
	"github.com/pkg/errors"
)

var getSyncFilesystemSourceTaskHandler = createFromConfigOnce(func(ctx context.Context, conf *config.Config) (*documentTask.SyncFilesystemSourceHandler, error) {
	filesystemSourceStore, err := getFilesystemSourceStoreFromConfig(ctx, conf)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	documentStore, err := getDocumentStoreFromConfig(ctx, conf)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	taskRunner, err := getTaskRunner(ctx, conf)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	return documentTask.NewSyncFilesystemSourceHandler(filesystemSourceStore, documentStore, taskRunner), nil
})
