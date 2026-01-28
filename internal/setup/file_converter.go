package setup

import (
	"context"
	"io"
	"path/filepath"
	"slices"

	"github.com/bornholm/corpus/internal/config"
	"github.com/bornholm/corpus/internal/core/port"
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

	return NewRoutedFileConverter(fileConverters...), nil
})

type RoutedFileConverter struct {
	supportedExtensions []string
	converters          []port.FileConverter
}

// Convert implements port.FileConverter.
func (c *RoutedFileConverter) Convert(ctx context.Context, filename string, r io.Reader) (io.ReadCloser, error) {
	ext := filepath.Ext(filename)
	for _, c := range c.converters {
		if !slices.Contains(c.SupportedExtensions(), ext) {
			continue
		}

		readCloser, err := c.Convert(ctx, filename, r)
		if err != nil {
			return nil, errors.WithStack(err)
		}

		return readCloser, nil
	}

	return nil, errors.WithStack(port.ErrNotSupported)
}

// SupportedExtensions implements port.FileConverter.
func (c *RoutedFileConverter) SupportedExtensions() []string {
	return c.supportedExtensions
}

func NewRoutedFileConverter(converters ...port.FileConverter) *RoutedFileConverter {
	supportedExtensions := make([]string, 0)
	for _, c := range converters {
		supportedExtensions = append(supportedExtensions, c.SupportedExtensions()...)
	}

	return &RoutedFileConverter{
		supportedExtensions: supportedExtensions,
		converters:          converters,
	}
}

var _ port.FileConverter = &RoutedFileConverter{}
