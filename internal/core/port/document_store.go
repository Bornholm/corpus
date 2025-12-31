package port

import (
	"context"
	"net/url"

	"github.com/bornholm/corpus/internal/core/model"
)

type DocumentStore interface {
	CountDocuments(ctx context.Context) (int64, error)
	GetSectionByID(ctx context.Context, id model.SectionID) (model.Section, error)
	SectionExists(ctx context.Context, id model.SectionID) (bool, error)
	GetDocumentByID(ctx context.Context, id model.DocumentID) (model.Document, error)
	SaveDocuments(ctx context.Context, documents ...model.Document) error
	DeleteDocumentBySource(ctx context.Context, source *url.URL) error
	DeleteDocumentByID(ctx context.Context, id model.DocumentID) error
	QueryDocuments(ctx context.Context, opts QueryDocumentsOptions) ([]model.Document, int64, error)
	GetCollectionByName(ctx context.Context, name string) (model.Collection, error)
	QueryCollections(ctx context.Context, opts QueryCollectionsOptions) ([]model.Collection, error)
	CreateCollection(ctx context.Context, name string) (model.Collection, error)
	UpdateCollection(ctx context.Context, id model.CollectionID, updates CollectionUpdates) (model.Collection, error)
	GetCollectionStats(ctx context.Context, id model.CollectionID) (*model.CollectionStats, error)
}

type QueryDocumentsOptions struct {
	Page           *int
	Limit          *int
	HeaderOnly     bool
	MatchingSource *url.URL
}

type QueryCollectionsOptions struct {
	Page       *int
	Limit      *int
	HeaderOnly bool
}

type CollectionUpdates struct {
	Name        *string
	Description *string
}
