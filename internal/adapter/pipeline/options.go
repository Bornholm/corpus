package pipeline

import (
	"context"

	"github.com/bornholm/corpus/internal/core/port"
)

type QueryTransformer interface {
	TransformQuery(ctx context.Context, query string) (string, error)
}

type QueryTransformerFunc func(ctx context.Context, query string) (string, error)

func (fn QueryTransformerFunc) TransformQuery(ctx context.Context, query string) (string, error) {
	return fn(ctx, query)
}

type ResultsTransformer interface {
	TransformResults(ctx context.Context, query string, results []*port.IndexSearchResult) ([]*port.IndexSearchResult, error)
}

type ResultsTransformerFunc func(ctx context.Context, query string, results []*port.IndexSearchResult) ([]*port.IndexSearchResult, error)

func (fn ResultsTransformerFunc) TransformResults(ctx context.Context, query string, results []*port.IndexSearchResult) ([]*port.IndexSearchResult, error) {
	return fn(ctx, query, results)
}

type Options struct {
	QueryTransformers   []QueryTransformer
	ResultsTransformers []ResultsTransformer
}

type OptionFunc func(opts *Options)

func NewOptions(funcs ...OptionFunc) *Options {
	opts := &Options{
		QueryTransformers:   make([]QueryTransformer, 0),
		ResultsTransformers: make([]ResultsTransformer, 0),
	}

	for _, fn := range funcs {
		fn(opts)
	}

	return opts
}

func WithQueryTransformers(transformers ...QueryTransformer) OptionFunc {
	return func(opts *Options) {
		opts.QueryTransformers = transformers
	}
}

func WithResultsTransformers(transformers ...ResultsTransformer) OptionFunc {
	return func(opts *Options) {
		opts.ResultsTransformers = transformers
	}
}
