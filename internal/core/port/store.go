package port

import (
	"context"
	"errors"
	"net/url"

	"github.com/bornholm/corpus/internal/core/model"
)

var (
	ErrNotFound = errors.New("not found")
)

type Store interface {
	CountDocuments(ctx context.Context) (int64, error)
	GetDocumentBySource(ctx context.Context, source *url.URL) (model.Document, error)
	GetSectionBySourceAndID(ctx context.Context, source *url.URL, id model.SectionID) (model.Section, error)
	SaveDocument(ctx context.Context, doc model.Document) error
	DeleteDocumentBySource(ctx context.Context, source *url.URL) error
	QueryDocuments(ctx context.Context, opts QueryDocumentsOptions) ([]*model.Document, int64, error)
	GetCollectionByName(ctx context.Context, name string) (model.Collection, error)
	QueryCollections(ctx context.Context, opts QueryCollectionsOptions) ([]model.Collection, error)
	CreateCollection(ctx context.Context, name string) (model.Collection, error)
	UpdateCollection(ctx context.Context, id model.CollectionID, updates CollectionUpdates) (model.Collection, error)
	GetCollectionStats(ctx context.Context, id model.CollectionID) (*model.CollectionStats, error)
}

type QueryDocumentsOptions struct {
	Page  *int
	Limit *int
}

type QueryCollectionsOptions struct {
	Page  *int
	Limit *int
}

type CollectionUpdates struct {
	Name        *string
	Description *string
}
