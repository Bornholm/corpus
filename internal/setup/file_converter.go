package setup

import (
	"context"

	"github.com/bornholm/corpus/internal/config"
	"github.com/bornholm/corpus/internal/core/port"
	"github.com/pkg/errors"
)

var FileConverter = NewRegistry[port.FileConverter]()

var getFileConverter = createFromConfigOnce(func(ctx context.Context, conf *config.Config) (port.FileConverter, error) {
	fileConverter, err := FileConverter.From(conf.FileConverter.URI)
	if err != nil {
		return nil, errors.Wrapf(err, "could not retrieve file converter for uri '%s'", conf.FileConverter.URI)
	}

	return fileConverter, nil
})
