package client

import (
	"context"
	"net/url"
	"strconv"

	"github.com/bornholm/corpus/internal/http/handler/api"
	"github.com/pkg/errors"
)

type QueryDocumentsOptions struct {
	Page        *int
	Limit       *int
	Collections []string
	Source      *url.URL
}

type QueryDocumentsOptionFunc func(opts *QueryDocumentsOptions)

func WithQueryDocumentsPage(page int) QueryDocumentsOptionFunc {
	return func(opts *QueryDocumentsOptions) {
		opts.Page = &page
	}
}

func WithQueryDocumentsLimit(limit int) QueryDocumentsOptionFunc {
	return func(opts *QueryDocumentsOptions) {
		opts.Limit = &limit
	}
}

func WithQueryDocumentsCollections(collections ...string) QueryDocumentsOptionFunc {
	return func(opts *QueryDocumentsOptions) {
		opts.Collections = collections
	}
}

func WithQueryDocumentsSource(source *url.URL) QueryDocumentsOptionFunc {
	return func(opts *QueryDocumentsOptions) {
		opts.Source = source
	}
}

func NewQueryDocumentsOptions(funcs ...QueryDocumentsOptionFunc) *QueryDocumentsOptions {
	opts := &QueryDocumentsOptions{
		Collections: make([]string, 0),
	}

	for _, fn := range funcs {
		fn(opts)
	}

	return opts
}

func (c *Client) QueryDocuments(ctx context.Context, funcs ...QueryDocumentsOptionFunc) ([]DocumentHeader, int64, error) {
	opts := NewQueryDocumentsOptions(funcs...)

	endpoint := &url.URL{
		Path: "/documents",
	}

	query := endpoint.Query()

	if len(opts.Collections) > 0 {
		for _, c := range opts.Collections {
			query.Add("collection", c)
		}
	}

	if opts.Source != nil {
		query.Set("source", opts.Source.String())
	}

	if opts.Page != nil {
		query.Set("page", strconv.FormatInt(int64(*opts.Page), 10))
	}

	if opts.Limit != nil {
		query.Set("limit", strconv.FormatInt(int64(*opts.Limit), 10))
	}

	endpoint.RawQuery = query.Encode()

	var res api.ListDocumentsResponse

	if err := c.jsonRequest(ctx, "GET", endpoint.String(), nil, nil, &res); err != nil {
		return nil, 0, errors.WithStack(err)
	}

	return res.Documents, res.Total, nil
}

func (c *Client) DeleteDocument(ctx context.Context, id string) error {
	endpoint := &url.URL{
		Path: "/documents",
	}

	endpoint = endpoint.JoinPath(id)

	if err := c.request(ctx, "DELETE", endpoint.String(), nil, nil, nil); err != nil {
		return errors.WithStack(err)
	}

	return nil
}
