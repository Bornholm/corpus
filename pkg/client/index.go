package client

import (
	"bytes"
	"context"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"

	"github.com/bornholm/corpus/internal/http/handler/api"
	"github.com/pkg/errors"
)

type IndexOptions struct {
	Collections []string
	ETag        string
	Source      *url.URL
}

type IndexOptionFunc func(opts *IndexOptions)

func WithIndexCollections(collections ...string) IndexOptionFunc {
	return func(opts *IndexOptions) {
		opts.Collections = collections
	}
}

func WithIndexSource(source *url.URL) IndexOptionFunc {
	return func(opts *IndexOptions) {
		opts.Source = source
	}
}

func WithIndexETag(etag string) IndexOptionFunc {
	return func(opts *IndexOptions) {
		opts.ETag = etag
	}
}

func NewIndexOptions(funcs ...IndexOptionFunc) *IndexOptions {
	opts := &IndexOptions{
		Collections: make([]string, 0),
	}

	for _, fn := range funcs {
		fn(opts)
	}

	return opts
}

func (c *Client) Index(ctx context.Context, filename string, r io.Reader, funcs ...IndexOptionFunc) (*Task, error) {
	opts := NewIndexOptions(funcs...)

	var body bytes.Buffer
	form := multipart.NewWriter(&body)

	fileWriter, err := form.CreateFormFile("file", filename)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	if _, err := io.Copy(fileWriter, r); err != nil {
		return nil, errors.WithStack(err)
	}

	for _, c := range opts.Collections {
		if err := form.WriteField("collection", c); err != nil {
			return nil, errors.WithStack(err)
		}
	}

	if opts.Source != nil {
		if err := form.WriteField("source", opts.Source.String()); err != nil {
			return nil, errors.WithStack(err)
		}
	}

	if opts.ETag != "" {
		if err := form.WriteField("etag", opts.ETag); err != nil {
			return nil, errors.WithStack(err)
		}
	}

	if err := form.Close(); err != nil {
		return nil, errors.WithStack(err)
	}

	var taskResponse api.ShowTaskResponse

	header := http.Header{}
	header.Set("Content-Type", form.FormDataContentType())

	if err := c.jsonRequest(ctx, "POST", "/index", header, &body, &taskResponse); err != nil {
		return nil, errors.WithStack(err)
	}

	return taskResponse.Task, nil
}

func (c *Client) IndexFile(ctx context.Context, path string, funcs ...IndexOptionFunc) (*Task, error) {
	filename := filepath.Base(path)

	file, err := os.Open(path)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	defer file.Close()

	task, err := c.Index(ctx, filename, file, funcs...)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	return task, nil
}
