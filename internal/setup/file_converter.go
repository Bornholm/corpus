package setup

import (
	"context"

	"github.com/bornholm/corpus/internal/config"
	"github.com/bornholm/corpus/internal/core/port"
	"github.com/bornholm/corpus/internal/fileconverter"
	"github.com/pkg/errors"
)

var FileConverter = NewRegistry[port.FileConverter]()

var getFileConverterFromConfig = createFromConfigOnce(func(ctx context.Context, conf *config.Config) (port.FileConverter, error) {
	fileConverters := make([]port.FileConverter, 0)
	for _, uri := range conf.FileConverter.URI {
		c, err := FileConverter.From(uri)
		if err != nil {
			return nil, errors.Wrapf(err, "could not retrieve file converter for uri '%s'", uri)
		}

		fileConverters = append(fileConverters, c)
	}

	var fileConverter port.FileConverter = fileconverter.NewRoutedFileConverter(fileConverters...)

	if conf.FileConverter.RateLimit.Enabled {
		fileConverter = fileconverter.NewRateLimitedFileConverter(fileConverter, conf.FileConverter.RateLimit.RequestInterval, conf.FileConverter.RateLimit.RequestMaxBurst)
	}

	if conf.FileConverter.MaxRetries != 0 {
		fileConverter = fileconverter.NewRetryFileConverter(fileConverter, conf.FileConverter.BaseBackoff, conf.FileConverter.MaxRetries)
	}

	return fileConverter, nil
})
