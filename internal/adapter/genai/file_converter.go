package genai

import (
	"context"
	"io"

	"github.com/bornholm/corpus/internal/core/port"
	"github.com/bornholm/genai/llm"
	"github.com/pkg/errors"
)

type FileConverter struct {
	client     llm.ExtractTextClient
	extensions []string
}

// Convert implements port.FileConverter.
func (f *FileConverter) Convert(ctx context.Context, filename string, r io.Reader) (io.ReadCloser, error) {
	res, err := f.client.ExtractText(ctx, llm.WithReader(r))
	if err != nil {
		return nil, errors.WithStack(err)
	}

	return io.NopCloser(res.Output()), nil
}

// SupportedExtensions implements port.FileConverter.
func (f *FileConverter) SupportedExtensions() []string {
	return f.extensions
}

func NewFileConverter(client llm.ExtractTextClient, extensions ...string) *FileConverter {
	return &FileConverter{client: client, extensions: extensions}
}

var _ port.FileConverter = &FileConverter{}
