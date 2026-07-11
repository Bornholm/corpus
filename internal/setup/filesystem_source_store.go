package setup

import (
	"context"

	"github.com/bornholm/corpus/internal/config"
	"github.com/bornholm/corpus/pkg/port"
	"github.com/pkg/errors"
)

var getFilesystemSourceStoreFromConfig = createFromConfigOnce(func(ctx context.Context, conf *config.Config) (port.FilesystemSourceStore, error) {
	store, err := getGormStoreFromConfig(ctx, conf)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return store, nil
})
