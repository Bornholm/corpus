package port

import (
	"context"
	"net/url"

	"github.com/bornholm/corpus/internal/core/model"
)

type Index interface {
	Index(ctx context.Context, document model.Document, funcs ...IndexOptionFunc) error
	DeleteBySource(ctx context.Context, source *url.URL) error
	Search(ctx context.Context, query string, opts IndexSearchOptions) ([]*IndexSearchResult, error)
}

type IndexOptions struct {
	OnProgress func(progress float32)
}

type IndexOptionFunc func(opts *IndexOptions)

func NewIndexOptions(funcs ...IndexOptionFunc) *IndexOptions {
	opts := &IndexOptions{}
	for _, fn := range funcs {
		fn(opts)
	}
	return opts
}

func WithIndexOnProgress(onProgress func(progress float32)) IndexOptionFunc {
	return func(opts *IndexOptions) {
		opts.OnProgress = onProgress
	}
}

type IndexSearchOptions struct {
	MaxResults  int
	Collections []model.CollectionID
}

type IndexSearchResult struct {
	Source   *url.URL
	Sections []model.SectionID
}
