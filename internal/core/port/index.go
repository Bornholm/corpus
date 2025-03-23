package port

import (
	"context"
	"net/url"

	"github.com/bornholm/corpus/internal/core/model"
)

type Index interface {
	Index(ctx context.Context, document model.Document) error
	DeleteBySource(ctx context.Context, source *url.URL) error
	Search(ctx context.Context, query string, opts IndexSearchOptions) ([]*IndexSearchResult, error)
}

type IndexSearchOptions struct {
	MaxResults  int
	Collections []model.CollectionID
}

type IndexSearchResult struct {
	Source   *url.URL
	Sections []model.SectionID
}
