package port

import (
	"context"
	"net/url"

	"github.com/bornholm/corpus/internal/core/model"
)

type DocumentStore interface {
	GetDocumentByID(ctx context.Context, id model.DocumentID) (model.PersistedDocument, error)
	SaveDocuments(ctx context.Context, documents ...model.OwnedDocument) error
	DeleteDocumentBySource(ctx context.Context, ownerID model.UserID, source *url.URL) error
	DeleteDocumentByID(ctx context.Context, id model.DocumentID) error
	QueryDocuments(ctx context.Context, opts QueryDocumentsOptions) ([]model.PersistedDocument, int64, error)

	QueryUserReadableDocuments(ctx context.Context, userID model.UserID, opts QueryDocumentsOptions) ([]model.PersistedDocument, int64, error)
	QueryUserWritableDocuments(ctx context.Context, userID model.UserID, opts QueryDocumentsOptions) ([]model.PersistedDocument, int64, error)

	CanReadDocument(ctx context.Context, userID model.UserID, documentID model.DocumentID) (bool, error)
	CanWriteDocument(ctx context.Context, userID model.UserID, documentID model.DocumentID) (bool, error)

	CountReadableDocuments(ctx context.Context, userID model.UserID) (int64, error)

	GetSectionByID(ctx context.Context, id model.SectionID) (model.Section, error)
	SectionExists(ctx context.Context, id model.SectionID) (bool, error)

	GetCollectionByID(ctx context.Context, id model.CollectionID) (model.PersistedCollection, error)
	QueryCollections(ctx context.Context, opts QueryCollectionsOptions) ([]model.PersistedCollection, error)
	CreateCollection(ctx context.Context, ownerID model.UserID, label string) (model.PersistedCollection, error)
	UpdateCollection(ctx context.Context, id model.CollectionID, updates CollectionUpdates) (model.PersistedCollection, error)
	GetCollectionStats(ctx context.Context, id model.CollectionID) (*model.CollectionStats, error)

	DeleteCollection(ctx context.Context, id model.CollectionID) error

	QueryUserReadableCollections(ctx context.Context, userID model.UserID, opts QueryCollectionsOptions) ([]model.PersistedCollection, int64, error)
	QueryUserWritableCollections(ctx context.Context, userID model.UserID, opts QueryCollectionsOptions) ([]model.PersistedCollection, int64, error)

	CanReadCollection(ctx context.Context, userID model.UserID, collectionID model.CollectionID) (bool, error)
	CanWriteCollection(ctx context.Context, userID model.UserID, collectionID model.CollectionID) (bool, error)
}

type QueryDocumentsOptions struct {
	Page  *int
	Limit *int

	HeaderOnly bool

	MatchingSource *url.URL
}

type QueryCollectionsOptions struct {
	Page  *int
	Limit *int

	IDs []model.CollectionID
}

type CollectionUpdates struct {
	Label       *string
	Description *string
}
