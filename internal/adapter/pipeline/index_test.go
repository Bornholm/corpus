package pipeline

import (
	"context"
	"net/url"
	"testing"
	"time"

	"github.com/bornholm/corpus/internal/core/model"
	"github.com/bornholm/corpus/internal/core/port"
	"github.com/bornholm/corpus/internal/markdown"
	"github.com/pkg/errors"
)

func TestIndex(t *testing.T) {
	index := NewIndex(
		WeightedIndexes{
			&mockIndex{}: 1,
			&mockIndex{
				indexErr: errors.New("Oh snap !"),
			}: 1,
			&mockIndex{
				indexErr: errors.New("Oh snap !"),
			}: 1,
		},
	)

	document, err := markdown.Parse([]byte(""))
	if err != nil {
		t.Fatalf("%+v", errors.WithStack(err))
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := index.Index(ctx, document); err == nil {
		t.Error("err should not be nil")
	}
}

type mockIndex struct {
	indexErr error
}

// DeleteBySource implements port.Index.
func (m *mockIndex) DeleteBySource(ctx context.Context, source *url.URL) error {
	return nil
}

// Index implements port.Index.
func (m *mockIndex) Index(ctx context.Context, document model.Document) error {
	return m.indexErr
}

// Search implements port.Index.
func (m *mockIndex) Search(ctx context.Context, query string, opts port.IndexSearchOptions) ([]*port.IndexSearchResult, error) {
	return []*port.IndexSearchResult{}, nil
}

var _ port.Index = &mockIndex{}
