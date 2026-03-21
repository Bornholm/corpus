package port

import (
	"context"
	"errors"
	"io"
)

var (
	ErrNotSupported = errors.New("not supported")
)

// FileConverter converts a given file to its markdown equivalent.
type FileConverter interface {
	SupportedExtensions() []string
	Convert(ctx context.Context, filename string, r io.Reader) (io.ReadCloser, error)
}
