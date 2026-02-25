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
	DeleteDocumentByID(ctx context.Context, ids ...model.DocumentID) error
	QueryDocuments(ctx context.Context, opts QueryDocumentsOptions) ([]model.PersistedDocument, int64, error)

	// QueryDocumentsByCollectionID retrieves all documents belonging to a specific collection.
	QueryDocumentsByCollectionID(ctx context.Context, collectionID model.CollectionID, opts QueryDocumentsOptions) ([]model.PersistedDocument, int64, error)

	QueryUserReadableDocuments(ctx context.Context, userID model.UserID, opts QueryDocumentsOptions) ([]model.PersistedDocument, int64, error)
	QueryUserWritableDocuments(ctx context.Context, userID model.UserID, opts QueryDocumentsOptions) ([]model.PersistedDocument, int64, error)

	CanReadDocument(ctx context.Context, userID model.UserID, documentID model.DocumentID) (bool, error)
	CanWriteDocument(ctx context.Context, userID model.UserID, documentID model.DocumentID) (bool, error)

	CountReadableDocuments(ctx context.Context, userID model.UserID) (int64, error)

	GetSectionByID(ctx context.Context, id model.SectionID) (model.Section, error)
	SectionExists(ctx context.Context, id model.SectionID) (bool, error)

	GetCollectionByID(ctx context.Context, id model.CollectionID, full bool) (model.PersistedCollection, error)
	QueryCollections(ctx context.Context, opts QueryCollectionsOptions) ([]model.PersistedCollection, error)
	CreateCollection(ctx context.Context, ownerID model.UserID, label string) (model.PersistedCollection, error)
	UpdateCollection(ctx context.Context, id model.CollectionID, updates CollectionUpdates) (model.PersistedCollection, error)
	GetCollectionStats(ctx context.Context, id model.CollectionID) (*model.CollectionStats, error)

	DeleteCollection(ctx context.Context, id model.CollectionID) error

	QueryUserReadableCollections(ctx context.Context, userID model.UserID, opts QueryCollectionsOptions) ([]model.PersistedCollection, int64, error)
	QueryUserWritableCollections(ctx context.Context, userID model.UserID, opts QueryCollectionsOptions) ([]model.PersistedCollection, int64, error)

	CanReadCollection(ctx context.Context, userID model.UserID, collectionID model.CollectionID) (bool, error)
	CanWriteCollection(ctx context.Context, userID model.UserID, collectionID model.CollectionID) (bool, error)

	// CreateCollectionShare shares a collection with a specific user at the given access level.
	// Only one share per (collection, user) pair is allowed; an existing share will be updated.
	CreateCollectionShare(ctx context.Context, collectionID model.CollectionID, userID model.UserID, level model.CollectionShareLevel) (model.PersistedCollectionShare, error)

	// DeleteCollectionShare removes a share by its ID.
	DeleteCollectionShare(ctx context.Context, shareID model.CollectionShareID) error

	// GetCollectionShares returns all shares for a given collection.
	GetCollectionShares(ctx context.Context, collectionID model.CollectionID) ([]model.PersistedCollectionShare, error)
}

type QueryDocumentsOptions struct {
	Page  *int
	Limit *int

	// Do not retrieve associations
	HeaderOnly bool

	// Filters

	// Documents matching the given source
	MatchingSource *url.URL

	// Documents without parent collection
	Orphaned *bool
}

type QueryCollectionsOptions struct {
	Page  *int
	Limit *int

	// Do not retrieve associations
	HeaderOnly bool

	// Filters

	// Collections with these ids
	IDs []model.CollectionID
}

type CollectionUpdates struct {
	Label       *string
	Description *string
}
