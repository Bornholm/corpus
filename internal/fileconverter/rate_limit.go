package fileconverter

import (
	"context"
	"io"
	"time"

	"github.com/bornholm/corpus/internal/core/port"
	"github.com/pkg/errors"
	"golang.org/x/time/rate"
)

type RateLimitedFileConverter struct {
	limiter       *rate.Limiter
	fileConverter port.FileConverter
}

// Convert implements [port.FileConverter].
func (c *RateLimitedFileConverter) Convert(ctx context.Context, filename string, r io.Reader) (io.ReadCloser, error) {
	if err := c.limiter.Wait(ctx); err != nil {
		return nil, errors.WithStack(err)
	}

	return c.fileConverter.Convert(ctx, filename, r)
}

// SupportedExtensions implements [port.FileConverter].
func (c *RateLimitedFileConverter) SupportedExtensions() []string {
	return c.fileConverter.SupportedExtensions()
}

func NewRateLimitedFileConverter(fileConverter port.FileConverter, interval time.Duration, maxBurst int) *RateLimitedFileConverter {
	return &RateLimitedFileConverter{
		limiter:       rate.NewLimiter(rate.Every(interval), maxBurst),
		fileConverter: fileConverter,
	}
}

var _ port.FileConverter = &RateLimitedFileConverter{}
