package cache

import (
	"context"
	"net/url"
	"time"

	"github.com/bornholm/corpus/internal/core/model"
	"github.com/bornholm/corpus/internal/core/port"
	"github.com/hashicorp/golang-lru/v2/expirable"
	"github.com/pkg/errors"
)

type DocumentStore struct {
	backend            port.DocumentStore
	documentCache      *MultiIndexCache[*CacheableDocument]
	collectionCache    *MultiIndexCache[*CacheableCollection]
	sectionCache       *MultiIndexCache[*CacheableSection]
	authorizationCache *expirable.LRU[string, bool]
	statCache          *expirable.LRU[string, int64]
}

// CanReadCollection implements [port.DocumentStore].
func (s *DocumentStore) CanReadCollection(ctx context.Context, userID model.UserID, collectionID model.CollectionID) (bool, error) {
	cacheKey := getCompositeCacheKey(userID, collectionID, "read", "collection")

	if authorized, exists := s.authorizationCache.Get(cacheKey); exists {
		return authorized, nil
	}

	authorized, err := s.backend.CanReadCollection(ctx, userID, collectionID)
	if err != nil {
		return false, err
	}

	s.authorizationCache.Add(cacheKey, authorized)

	return authorized, nil
}

// CanReadDocument implements [port.DocumentStore].
func (s *DocumentStore) CanReadDocument(ctx context.Context, userID model.UserID, documentID model.DocumentID) (bool, error) {
	cacheKey := getCompositeCacheKey(userID, documentID, "read", "document")

	if authorized, exists := s.authorizationCache.Get(cacheKey); exists {
		return authorized, nil
	}

	authorized, err := s.backend.CanReadDocument(ctx, userID, documentID)
	if err != nil {
		return false, err
	}

	s.authorizationCache.Add(cacheKey, authorized)

	return authorized, nil
}

// CanWriteCollection implements [port.DocumentStore].
func (s *DocumentStore) CanWriteCollection(ctx context.Context, userID model.UserID, collectionID model.CollectionID) (bool, error) {
	cacheKey := getCompositeCacheKey(userID, collectionID, "write", "collection")

	if authorized, exists := s.authorizationCache.Get(cacheKey); exists {
		return authorized, nil
	}

	authorized, err := s.backend.CanWriteCollection(ctx, userID, collectionID)
	if err != nil {
		return false, err
	}

	s.authorizationCache.Add(cacheKey, authorized)

	return authorized, nil
}

// CanWriteDocument implements [port.DocumentStore].
func (s *DocumentStore) CanWriteDocument(ctx context.Context, userID model.UserID, documentID model.DocumentID) (bool, error) {
	cacheKey := getCompositeCacheKey(userID, documentID, "write", "document")

	if authorized, exists := s.authorizationCache.Get(cacheKey); exists {
		return authorized, nil
	}

	authorized, err := s.backend.CanWriteDocument(ctx, userID, documentID)
	if err != nil {
		return false, err
	}

	s.authorizationCache.Add(cacheKey, authorized)

	return authorized, nil
}

func getReadableDocumentsCountCacheKey(userID model.UserID) string {
	return getCompositeCacheKey(userID, "readable", "document")
}

// CountReadableDocuments implements [port.DocumentStore].
func (s *DocumentStore) CountReadableDocuments(ctx context.Context, userID model.UserID) (int64, error) {
	cacheKey := getReadableDocumentsCountCacheKey(userID)

	if count, exists := s.statCache.Get(cacheKey); exists {
		return count, nil
	}

	count, err := s.backend.CountReadableDocuments(ctx, userID)
	if err != nil {
		return 0, err
	}

	s.statCache.Add(cacheKey, count)

	return count, nil
}

// CreateCollection implements [port.DocumentStore].
func (s *DocumentStore) CreateCollection(ctx context.Context, ownerID model.UserID, label string) (model.PersistedCollection, error) {
	return s.backend.CreateCollection(ctx, ownerID, label)
}

// CreateCollectionShare implements [port.DocumentStore].
func (s *DocumentStore) CreateCollectionShare(ctx context.Context, collectionID model.CollectionID, userID model.UserID, level model.CollectionShareLevel) (model.PersistedCollectionShare, error) {
	share, err := s.backend.CreateCollectionShare(ctx, collectionID, userID, level)
	if err != nil {
		return nil, err
	}

	// Invalidate authorization cache entries for this user+collection pair
	s.authorizationCache.Remove(getCompositeCacheKey(userID, collectionID, "read", "collection"))
	s.authorizationCache.Remove(getCompositeCacheKey(userID, collectionID, "write", "collection"))

	// Invalidate the readable document count for the user who was granted access
	s.statCache.Remove(getReadableDocumentsCountCacheKey(userID))

	return share, nil
}

// DeleteCollectionShare implements [port.DocumentStore].
func (s *DocumentStore) DeleteCollectionShare(ctx context.Context, shareID model.CollectionShareID) error {
	// We can't know which user/collection this was for without fetching first,
	// so purge the entire authorization and stat caches to ensure consistency.
	defer s.authorizationCache.Purge()
	defer s.statCache.Purge()

	return s.backend.DeleteCollectionShare(ctx, shareID)
}

// GetCollectionShares implements [port.DocumentStore].
func (s *DocumentStore) GetCollectionShares(ctx context.Context, collectionID model.CollectionID) ([]model.PersistedCollectionShare, error) {
	return s.backend.GetCollectionShares(ctx, collectionID)
}

// DeleteCollection implements [port.DocumentStore].
func (s *DocumentStore) DeleteCollection(ctx context.Context, id model.CollectionID) error {
	defer func() {
		s.statCache.Purge()
		s.collectionCache.Remove(string(id))
	}()

	return s.backend.DeleteCollection(ctx, id)
}

// DeleteDocumentByID implements [port.DocumentStore].
func (s *DocumentStore) DeleteDocumentByID(ctx context.Context, ids ...model.DocumentID) error {
	defer func() {
		s.statCache.Purge()
		for _, id := range ids {
			s.documentCache.Remove(string(id))
		}
	}()

	return s.backend.DeleteDocumentByID(ctx, ids...)
}

// DeleteDocumentBySource implements [port.DocumentStore].
func (s *DocumentStore) DeleteDocumentBySource(ctx context.Context, ownerID model.UserID, source *url.URL) error {
	defer func() {
		s.statCache.Remove(getReadableDocumentsCountCacheKey(ownerID))
		s.documentCache.Remove(getCompositeCacheKey(ownerID, source.String()))
	}()

	return s.backend.DeleteDocumentBySource(ctx, ownerID, source)
}

// GetCollectionByID implements [port.DocumentStore].
func (s *DocumentStore) GetCollectionByID(ctx context.Context, id model.CollectionID, full bool) (model.PersistedCollection, error) {
	cachedCollection, exists := s.collectionCache.Get(getCompositeCacheKey(id, full))
	if exists {
		return cachedCollection, nil
	}

	collection, err := s.backend.GetCollectionByID(ctx, id, full)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	s.collectionCache.Add(NewCacheableCollection(collection))

	return collection, nil
}

// GetCollectionStats implements [port.DocumentStore].
func (s *DocumentStore) GetCollectionStats(ctx context.Context, id model.CollectionID) (*model.CollectionStats, error) {
	return s.backend.GetCollectionStats(ctx, id)
}

// GetDocumentByID implements [port.DocumentStore].
func (s *DocumentStore) GetDocumentByID(ctx context.Context, id model.DocumentID) (model.PersistedDocument, error) {
	cachedDocument, exists := s.documentCache.Get(string(id))
	if exists {
		return cachedDocument, nil
	}

	document, err := s.backend.GetDocumentByID(ctx, id)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	s.documentCache.Add(NewCacheableDocument(document))

	return document, nil
}

// GetSectionByID implements [port.DocumentStore].
func (s *DocumentStore) GetSectionByID(ctx context.Context, id model.SectionID) (model.Section, error) {
	cachedSection, exists := s.sectionCache.Get(string(id))
	if exists {
		return cachedSection, nil
	}

	section, err := s.backend.GetSectionByID(ctx, id)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	s.sectionCache.Add(NewCacheableSection(section))

	return section, nil
}

// QueryCollections implements [port.DocumentStore].
func (s *DocumentStore) QueryCollections(ctx context.Context, opts port.QueryCollectionsOptions) ([]model.PersistedCollection, error) {
	return s.backend.QueryCollections(ctx, opts)
}

// QueryDocuments implements [port.DocumentStore].
func (s *DocumentStore) QueryDocuments(ctx context.Context, opts port.QueryDocumentsOptions) ([]model.PersistedDocument, int64, error) {
	return s.backend.QueryDocuments(ctx, opts)
}

// QueryUserReadableCollections implements [port.DocumentStore].
func (s *DocumentStore) QueryUserReadableCollections(ctx context.Context, userID model.UserID, opts port.QueryCollectionsOptions) ([]model.PersistedCollection, int64, error) {
	return s.backend.QueryUserReadableCollections(ctx, userID, opts)
}

// QueryUserReadableDocuments implements [port.DocumentStore].
func (s *DocumentStore) QueryUserReadableDocuments(ctx context.Context, userID model.UserID, opts port.QueryDocumentsOptions) ([]model.PersistedDocument, int64, error) {
	return s.backend.QueryUserReadableDocuments(ctx, userID, opts)
}

// QueryUserWritableCollections implements [port.DocumentStore].
func (s *DocumentStore) QueryUserWritableCollections(ctx context.Context, userID model.UserID, opts port.QueryCollectionsOptions) ([]model.PersistedCollection, int64, error) {
	return s.backend.QueryUserWritableCollections(ctx, userID, opts)
}

// QueryUserWritableDocuments implements [port.DocumentStore].
func (s *DocumentStore) QueryUserWritableDocuments(ctx context.Context, userID model.UserID, opts port.QueryDocumentsOptions) ([]model.PersistedDocument, int64, error) {
	return s.backend.QueryUserWritableDocuments(ctx, userID, opts)
}

// SaveDocuments implements [port.DocumentStore].
func (s *DocumentStore) SaveDocuments(ctx context.Context, documents ...model.OwnedDocument) error {
	defer func() {
		for _, d := range documents {
			s.statCache.Remove(getReadableDocumentsCountCacheKey(d.Owner().ID()))
			s.documentCache.Remove(string(d.ID()))
		}
	}()

	return s.backend.SaveDocuments(ctx, documents...)
}

// SectionExists implements [port.DocumentStore].
func (s *DocumentStore) SectionExists(ctx context.Context, id model.SectionID) (bool, error) {
	return s.backend.SectionExists(ctx, id)
}

// UpdateCollection implements [port.DocumentStore].
func (s *DocumentStore) UpdateCollection(ctx context.Context, id model.CollectionID, updates port.CollectionUpdates) (model.PersistedCollection, error) {
	defer s.collectionCache.Remove(string(id))

	return s.backend.UpdateCollection(ctx, id, updates)
}

func NewDocumentStore(backend port.DocumentStore, size int, ttl time.Duration) *DocumentStore {
	return &DocumentStore{
		backend:            backend,
		documentCache:      NewMultiIndexCache[*CacheableDocument](size, ttl),
		collectionCache:    NewMultiIndexCache[*CacheableCollection](size, ttl),
		sectionCache:       NewMultiIndexCache[*CacheableSection](size, ttl),
		authorizationCache: expirable.NewLRU[string, bool](size, nil, ttl),
		statCache:          expirable.NewLRU[string, int64](size, nil, ttl),
	}
}

var _ port.DocumentStore = &DocumentStore{}
