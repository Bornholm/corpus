package genai

import (
	"context"
	"io"

	"github.com/bornholm/corpus/internal/core/port"
	"github.com/bornholm/genai/extract"
	"github.com/pkg/errors"
)

type FileConverter struct {
	extract    extract.TextClient
	extensions []string
}

// Convert implements port.FileConverter.
func (f *FileConverter) Convert(ctx context.Context, filename string, r io.Reader) (io.ReadCloser, error) {
	res, err := f.extract.Text(ctx, extract.WithReader(r), extract.WithFilename(filename))
	if err != nil {
		return nil, errors.WithStack(err)
	}

	return io.NopCloser(res.Output()), nil
}

// SupportedExtensions implements port.FileConverter.
func (f *FileConverter) SupportedExtensions() []string {
	return f.extensions
}

func NewFileConverter(extract extract.TextClient, extensions ...string) *FileConverter {
	return &FileConverter{extract: extract, extensions: extensions}
}

var _ port.FileConverter = &FileConverter{}
