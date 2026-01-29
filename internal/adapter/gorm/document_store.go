package gorm

import (
	"context"
	"log/slog"
	"net/url"
	"slices"

	"github.com/bornholm/corpus/internal/core/model"
	"github.com/bornholm/corpus/internal/core/port"
	"github.com/ncruces/go-sqlite3"
	"github.com/pkg/errors"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// DeleteDocumentByID implements port.DocumentStore.
func (s *Store) DeleteDocumentByID(ctx context.Context, ids ...model.DocumentID) error {
	err := s.withRetry(ctx, func(ctx context.Context, db *gorm.DB) error {
		if err := db.Model(&Section{}).Delete("document_id in ?", ids).Error; err != nil {
			return errors.WithStack(err)
		}

		if err := db.Model(&Document{}).Delete("id in ?", ids).Error; err != nil {
			return errors.WithStack(err)
		}

		return nil
	}, sqlite3.LOCKED, sqlite3.BUSY)
	if err != nil {
		return errors.WithStack(err)
	}

	return nil
}

// SectionExists implements port.DocumentStore.
func (s *Store) SectionExists(ctx context.Context, id model.SectionID) (bool, error) {
	var exists bool

	err := s.withRetry(ctx, func(ctx context.Context, db *gorm.DB) error {
		var count int64
		if err := db.Model(&Section{}).Where("id = ?", string(id)).Count(&count).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return errors.WithStack(port.ErrNotFound)
			}

			return errors.WithStack(err)
		}

		exists = count > 0

		return nil
	}, sqlite3.LOCKED, sqlite3.BUSY)
	if err != nil {
		return false, errors.WithStack(err)
	}

	return exists, nil
}

// GetDocumentByID implements port.DocumentStore.
func (s *Store) GetDocumentByID(ctx context.Context, id model.DocumentID) (model.PersistedDocument, error) {
	var document Document

	err := s.withRetry(ctx, func(ctx context.Context, db *gorm.DB) error {
		if err := db.Preload(clause.Associations).Preload("Sections", "parent_id is null").First(&document, "id = ?", id).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return errors.WithStack(port.ErrNotFound)
			}

			return errors.WithStack(err)
		}

		return nil
	}, sqlite3.LOCKED, sqlite3.BUSY)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	return &wrappedDocument{&document}, nil
}

// GetCollectionByID implements [port.DocumentStore].
func (s *Store) GetCollectionByID(ctx context.Context, id model.CollectionID, full bool) (model.PersistedCollection, error) {
	var collection Collection

	err := s.withRetry(ctx, func(ctx context.Context, db *gorm.DB) error {
		query := db.Preload("Owner")
		if full {
			query = query.Preload(clause.Associations)
		}
		if err := query.First(&collection, "id = ?", id).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return errors.WithStack(port.ErrNotFound)
			}

			return errors.WithStack(err)
		}

		return nil
	}, sqlite3.LOCKED, sqlite3.BUSY)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	return &wrappedCollection{&collection}, nil
}

// GetCollectionStats implements port.DocumentStore.
func (s *Store) GetCollectionStats(ctx context.Context, id model.CollectionID) (*model.CollectionStats, error) {
	db, err := s.getDatabase(ctx)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	var collection *Collection

	if err := db.Find(&collection, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.WithStack(port.ErrNotFound)
		}

		return nil, errors.WithStack(err)
	}

	stats := &model.CollectionStats{
		TotalDocuments: db.Model(&collection).Association("Documents").Count(),
	}

	return stats, nil
}

// CreateCollection implements port.DocumentStore.
func (s *Store) CreateCollection(ctx context.Context, ownerID model.UserID, label string) (model.PersistedCollection, error) {
	var collection model.PersistedCollection
	err := s.withRetry(ctx, func(ctx context.Context, db *gorm.DB) error {
		coll := Collection{
			ID:      string(model.NewCollectionID()),
			OwnerID: string(ownerID),
			Label:   label,
		}

		if err := db.Create(&coll).Error; err != nil {
			return errors.WithStack(err)
		}

		collection = &wrappedCollection{&coll}

		return nil
	}, sqlite3.BUSY, sqlite3.LOCKED)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	return collection, nil
}

// UpdateCollection implements port.DocumentStore.
func (s *Store) UpdateCollection(ctx context.Context, id model.CollectionID, updates port.CollectionUpdates) (model.PersistedCollection, error) {
	var collection Collection

	err := s.withRetry(ctx, func(ctx context.Context, db *gorm.DB) error {
		// First, find the existing collection
		if err := db.First(&collection, "id = ?", id).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return errors.WithStack(port.ErrNotFound)
			}
			return errors.WithStack(err)
		}

		// Prepare updates map
		updateFields := make(map[string]interface{})

		if updates.Label != nil {
			updateFields["label"] = *updates.Label
		}

		if updates.Description != nil {
			updateFields["description"] = *updates.Description
		}

		// Only perform update if there are fields to update
		if len(updateFields) > 0 {
			if err := db.Model(&collection).Updates(updateFields).Error; err != nil {
				return errors.WithStack(err)
			}
		}

		// Reload the collection to get the updated values
		if err := db.Preload(clause.Associations).First(&collection, "id = ?", id).Error; err != nil {
			return errors.WithStack(err)
		}

		return nil
	}, sqlite3.LOCKED, sqlite3.BUSY)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	return &wrappedCollection{&collection}, nil
}

// DeleteCollection implements [port.DocumentStore].
func (s *Store) DeleteCollection(ctx context.Context, id model.CollectionID) error {
	err := s.withRetry(ctx, func(ctx context.Context, db *gorm.DB) error {

		if err := db.Model(&Collection{
			ID: string(id),
		}).Association("PublicShares").Clear(); err != nil {
			return errors.WithStack(err)
		}

		if err := db.Model(&Collection{}).Delete("id = ?", id).Error; err != nil {
			return errors.WithStack(err)
		}

		return nil
	}, sqlite3.LOCKED, sqlite3.BUSY)
	if err != nil {
		return errors.WithStack(err)
	}

	return nil
}

// QueryCollections implements port.DocumentStore.
func (s *Store) QueryCollections(ctx context.Context, opts port.QueryCollectionsOptions) ([]model.PersistedCollection, error) {
	db, err := s.getDatabase(ctx)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	query := db.Model(&Collection{})

	if opts.IDs != nil {
		rawCollectionIDs := slices.Collect(func(yield func(string) bool) {
			for _, id := range opts.IDs {
				if !yield(string(id)) {
					return
				}
			}
		})
		query = query.Where("id in ?", rawCollectionIDs)
	}

	if opts.Page != nil {
		limit := 100
		if opts.Limit != nil {
			limit = *opts.Limit
		}

		query = query.Offset(*opts.Page * limit)
	}

	if opts.Limit != nil {
		query = query.Limit(*opts.Limit)
	}

	if !opts.HeaderOnly {
		query = query.Preload(clause.Associations)
	}

	var collections []*Collection

	if err := query.Find(&collections).Error; err != nil {
		return nil, errors.WithStack(err)
	}

	wrappedCollections := make([]model.PersistedCollection, 0, len(collections))
	for _, c := range collections {
		wrappedCollections = append(wrappedCollections, &wrappedCollection{c})
	}

	return wrappedCollections, nil
}

// CountReadableDocuments implements port.DocumentStore.
func (s *Store) CountReadableDocuments(ctx context.Context, userID model.UserID) (int64, error) {
	db, err := s.getDatabase(ctx)
	if err != nil {
		return 0, errors.WithStack(err)
	}

	var total int64

	if err := db.Model(&Document{}).Where("owner_id = ?", userID).Count(&total).Error; err != nil {
		return 0, errors.WithStack(err)
	}

	return total, nil
}

// GetSectionByID implements port.DocumentStore.
func (s *Store) GetSectionByID(ctx context.Context, id model.SectionID) (model.Section, error) {
	var section Section

	err := s.withRetry(ctx, func(ctx context.Context, db *gorm.DB) error {
		if err := db.Model(&section).Preload(clause.Associations).First(&section, "id = ?", id).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return errors.WithStack(port.ErrNotFound)
			}

			return errors.WithStack(err)
		}

		return nil
	}, sqlite3.LOCKED, sqlite3.BUSY)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	return &wrappedSection{&section}, nil
}

// DeleteDocumentBySource implements port.DocumentStore.
func (s *Store) DeleteDocumentBySource(ctx context.Context, ownerID model.UserID, source *url.URL) error {
	if source == nil {
		return errors.WithStack(ErrMissingSource)
	}

	err := s.withRetry(ctx, func(ctx context.Context, db *gorm.DB) error {
		var doc Document
		if err := db.First(&doc, "source = ? and owner_id = ?", source.String(), ownerID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil
			}

			return errors.WithStack(err)
		}

		if err := db.Select(clause.Associations).Delete(&doc).Error; err != nil {
			return errors.WithStack(err)
		}

		return nil
	}, sqlite3.LOCKED, sqlite3.BUSY)
	if err != nil {
		return errors.WithStack(err)
	}

	return nil
}

// QueryDocuments implements port.DocumentStore.
func (s *Store) QueryDocuments(ctx context.Context, opts port.QueryDocumentsOptions) ([]model.PersistedDocument, int64, error) {
	var (
		documents []*Document
		total     int64
	)

	err := s.withRetry(ctx, func(ctx context.Context, db *gorm.DB) error {
		limit := 10
		if opts.Limit != nil {
			limit = *opts.Limit
		}

		page := 0
		if opts.Page != nil {
			page = *opts.Page
		}

		if err := db.Model(&Document{}).Count(&total).Error; err != nil {
			return errors.WithStack(err)
		}

		query := db.Limit(limit).Offset(page * limit)

		if !opts.HeaderOnly {
			query = query.Preload(clause.Associations).Preload("Sections")
		} else {
			query = query.Omit(clause.Associations).Select("ID", "CreatedAt", "UpdatedAt", "Source", "ETag")
		}

		if opts.MatchingSource != nil {
			query = query.Where("source = ?", opts.MatchingSource.String())
		}

		if opts.Orphaned != nil {
			if *opts.Orphaned {
				// Find documents that have no collections attached
				query = query.Where("id NOT IN (SELECT document_id FROM documents_collections)")
			} else {
				// Find documents that have at least one collection attached
				query = query.Where("id IN (SELECT document_id FROM documents_collections)")
			}
		}

		if err := query.Find(&documents).Error; err != nil {
			return errors.WithStack(err)
		}

		return nil
	}, sqlite3.BUSY, sqlite3.LOCKED)
	if err != nil {
		return nil, total, errors.WithStack(err)
	}

	wrappedDocuments := make([]model.PersistedDocument, 0, len(documents))
	for _, d := range documents {
		wrappedDocuments = append(wrappedDocuments, &wrappedDocument{d})
	}

	return wrappedDocuments, total, nil
}

// QueryUserReadableDocuments implements port.DocumentStore.
func (s *Store) QueryUserReadableDocuments(ctx context.Context, userID model.UserID, opts port.QueryDocumentsOptions) ([]model.PersistedDocument, int64, error) {
	var (
		documents []*Document
		total     int64
	)

	err := s.withRetry(ctx, func(ctx context.Context, db *gorm.DB) error {
		query := db.Model(&Document{})

		query = query.Where("owner_id = ?", userID)

		if opts.MatchingSource != nil {
			query = query.Where("source = ?", opts.MatchingSource.String())
		}

		if opts.Orphaned != nil {
			if *opts.Orphaned {
				// Find documents that have no collections attached
				query = query.Where("id NOT IN (SELECT document_id FROM documents_collections)")
			} else {
				// Find documents that have at least one collection attached
				query = query.Where("id IN (SELECT document_id FROM documents_collections)")
			}
		}

		if err := query.Count(&total).Error; err != nil {
			return errors.WithStack(err)
		}

		limit := 10
		if opts.Limit != nil {
			limit = *opts.Limit
		}

		page := 0
		if opts.Page != nil {
			page = *opts.Page
		}

		query = query.Limit(limit).Offset(page * limit)

		if !opts.HeaderOnly {
			query = query.Preload(clause.Associations).Preload("Sections")
		} else {
			query = query.Omit(clause.Associations).Select("ID", "CreatedAt", "UpdatedAt", "Source", "ETag")
		}

		if err := query.Find(&documents).Error; err != nil {
			return errors.WithStack(err)
		}

		return nil
	}, sqlite3.BUSY, sqlite3.LOCKED)
	if err != nil {
		return nil, total, errors.WithStack(err)
	}

	wrappedDocuments := make([]model.PersistedDocument, 0, len(documents))
	for _, d := range documents {
		wrappedDocuments = append(wrappedDocuments, &wrappedDocument{d})
	}

	return wrappedDocuments, total, nil
}

// QueryUserWritableDocuments implements port.DocumentStore.
func (s *Store) QueryUserWritableDocuments(ctx context.Context, userID model.UserID, opts port.QueryDocumentsOptions) ([]model.PersistedDocument, int64, error) {
	var (
		documents []*Document
		total     int64
	)

	err := s.withRetry(ctx, func(ctx context.Context, db *gorm.DB) error {
		query := db.Model(&Document{})

		query = query.Where("owner_id = ?", userID)

		if opts.MatchingSource != nil {
			query = query.Where("source = ?", opts.MatchingSource.String())
		}

		if opts.Orphaned != nil {
			if *opts.Orphaned {
				// Find documents that have no collections attached
				query = query.Where("id NOT IN (SELECT document_id FROM documents_collections)")
			} else {
				// Find documents that have at least one collection attached
				query = query.Where("id IN (SELECT document_id FROM documents_collections)")
			}
		}

		if err := query.Count(&total).Error; err != nil {
			return errors.WithStack(err)
		}

		limit := 10
		if opts.Limit != nil {
			limit = *opts.Limit
		}

		page := 0
		if opts.Page != nil {
			page = *opts.Page
		}

		query = query.Limit(limit).Offset(page * limit)

		if !opts.HeaderOnly {
			query = query.Preload(clause.Associations).Preload("Sections")
		} else {
			query = query.Omit(clause.Associations).Select("ID", "CreatedAt", "UpdatedAt", "Source", "ETag")
		}

		if err := query.Find(&documents).Error; err != nil {
			return errors.WithStack(err)
		}

		return nil
	}, sqlite3.BUSY, sqlite3.LOCKED)
	if err != nil {
		return nil, total, errors.WithStack(err)
	}

	wrappedDocuments := make([]model.PersistedDocument, 0, len(documents))
	for _, d := range documents {
		wrappedDocuments = append(wrappedDocuments, &wrappedDocument{d})
	}

	return wrappedDocuments, total, nil
}

// SaveDocuments implements port.DocumentStore.
func (s *Store) SaveDocuments(ctx context.Context, documents ...model.OwnedDocument) error {
	for _, doc := range documents {
		err := s.withRetry(ctx, func(ctx context.Context, db *gorm.DB) error {
			source := doc.Source()
			if source == nil {
				return errors.WithStack(ErrMissingSource)
			}

			var existing Document
			if err := db.First(&existing, "source = ?", source.String()).Error; err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
				return errors.WithStack(err)
			}

			if existing.ID != "" {
				if err := db.Delete(&existing).Error; err != nil {
					return errors.WithStack(err)
				}
			}

			document, err := fromDocument(doc)
			if err != nil {
				return errors.WithStack(err)
			}

			if res := db.Omit("Sections").Create(document); res.Error != nil {
				return errors.WithStack(res.Error)
			}

			var createSection func(s *Section) error
			createSection = func(s *Section) error {
				err := db.
					Clauses(clause.OnConflict{
						Columns:   []clause.Column{{Name: "id"}},
						UpdateAll: true,
					}).
					Omit("Sections", "Parent", "Document").Create(s).Error
				if err != nil {
					return errors.WithStack(err)
				}

				for _, ss := range s.Sections {
					if err := createSection(ss); err != nil {
						return errors.WithStack(err)
					}

					err := db.Model(&Section{}).
						Where("id = ?", ss.ID).
						Updates(map[string]any{
							"parent_id":   s.ID,
							"document_id": s.DocumentID,
						}).
						Error
					if err != nil {
						return errors.WithStack(err)
					}
				}

				return nil
			}

			for _, s := range document.Sections {
				if err := createSection(s); err != nil {
					return errors.WithStack(err)
				}
			}

			return nil
		}, sqlite3.LOCKED, sqlite3.BUSY)
		if err != nil {
			return errors.WithStack(err)
		}
	}

	return nil
}

// QueryUserWritableCollections implements [port.DocumentStore].
func (s *Store) QueryUserWritableCollections(ctx context.Context, userID model.UserID, opts port.QueryCollectionsOptions) ([]model.PersistedCollection, int64, error) {
	var (
		collections []*Collection
		total       int64
	)

	err := s.withRetry(ctx, func(ctx context.Context, db *gorm.DB) error {
		query := db.Model(&Collection{}).Where("owner_id = ?", string(userID))

		// Apply ID filtering if specified
		if opts.IDs != nil {
			rawCollectionIDs := slices.Collect(func(yield func(string) bool) {
				for _, id := range opts.IDs {
					if !yield(string(id)) {
						return
					}
				}
			})
			query = query.Where("id in ?", rawCollectionIDs)
		}

		// Get total count before applying pagination
		if err := query.Count(&total).Error; err != nil {
			return errors.WithStack(err)
		}

		// Apply pagination
		if opts.Page != nil {
			limit := 100
			if opts.Limit != nil {
				limit = *opts.Limit
			}
			query = query.Offset(*opts.Page * limit)
		}

		if opts.Limit != nil {
			query = query.Limit(*opts.Limit)
		}

		// Execute the query
		if err := query.Find(&collections).Error; err != nil {
			return errors.WithStack(err)
		}

		return nil
	}, sqlite3.LOCKED, sqlite3.BUSY)
	if err != nil {
		return nil, 0, errors.WithStack(err)
	}

	wrappedCollections := make([]model.PersistedCollection, 0, len(collections))
	for _, c := range collections {
		wrappedCollections = append(wrappedCollections, &wrappedCollection{c})
	}

	return wrappedCollections, total, nil
}

// QueryUserReadableCollections implements [port.DocumentStore].
func (s *Store) QueryUserReadableCollections(ctx context.Context, userID model.UserID, opts port.QueryCollectionsOptions) ([]model.PersistedCollection, int64, error) {
	var (
		collections []*Collection
		total       int64
	)

	err := s.withRetry(ctx, func(ctx context.Context, db *gorm.DB) error {
		// For now, users can only read collections they own
		// In the future, this could be extended to include shared collections
		query := db.Model(&Collection{}).Where("owner_id = ?", string(userID))

		// Apply ID filtering if specified
		if opts.IDs != nil {
			rawCollectionIDs := slices.Collect(func(yield func(string) bool) {
				for _, id := range opts.IDs {
					if !yield(string(id)) {
						return
					}
				}
			})
			query = query.Where("id in ?", rawCollectionIDs)
		}

		// Get total count before applying pagination
		if err := query.Count(&total).Error; err != nil {
			return errors.WithStack(err)
		}

		// Apply pagination
		if opts.Page != nil {
			limit := 100
			if opts.Limit != nil {
				limit = *opts.Limit
			}
			query = query.Offset(*opts.Page * limit)
		}

		if opts.Limit != nil {
			query = query.Limit(*opts.Limit)
		}

		// Execute the query
		if err := query.Find(&collections).Error; err != nil {
			return errors.WithStack(err)
		}

		return nil
	}, sqlite3.LOCKED, sqlite3.BUSY)
	if err != nil {
		return nil, 0, errors.WithStack(err)
	}

	wrappedCollections := make([]model.PersistedCollection, 0, len(collections))
	for _, c := range collections {
		wrappedCollections = append(wrappedCollections, &wrappedCollection{c})
	}

	return wrappedCollections, total, nil
}

// CanReadCollection implements [port.DocumentStore].
func (s *Store) CanReadCollection(ctx context.Context, userID model.UserID, collectionID model.CollectionID) (bool, error) {
	slog.DebugContext(ctx, "checking collection read permission",
		slog.String("user_id", string(userID)),
		slog.String("collection_id", string(collectionID)))

	var canRead bool
	err := s.withRetry(ctx, func(ctx context.Context, db *gorm.DB) error {
		var collection Collection
		if err := db.First(&collection, "id = ? AND owner_id = ?", string(collectionID), string(userID)).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				canRead = false
				return errors.WithStack(port.ErrNotFound)
			}
			return errors.WithStack(err)
		}
		canRead = true
		return nil
	}, sqlite3.LOCKED, sqlite3.BUSY)

	if err != nil {
		slog.ErrorContext(ctx, "failed to check collection read permission", slog.Any("error", err))
		return false, errors.WithStack(err)
	}

	slog.DebugContext(ctx, "collection read permission result", slog.Bool("can_read", canRead))
	return canRead, nil
}

// CanReadDocument implements [port.DocumentStore].
func (s *Store) CanReadDocument(ctx context.Context, userID model.UserID, documentID model.DocumentID) (bool, error) {
	slog.DebugContext(ctx, "checking document read permission",
		slog.String("user_id", string(userID)),
		slog.String("document_id", string(documentID)))

	var canRead bool
	err := s.withRetry(ctx, func(ctx context.Context, db *gorm.DB) error {
		var document Document
		if err := db.First(&document, "id = ? AND owner_id = ?", string(documentID), string(userID)).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				canRead = false
				return errors.WithStack(port.ErrNotFound)
			}
			return errors.WithStack(err)
		}
		canRead = true
		return nil
	}, sqlite3.LOCKED, sqlite3.BUSY)

	if err != nil {
		slog.ErrorContext(ctx, "failed to check document read permission", slog.Any("error", err))
		return false, errors.WithStack(err)
	}

	slog.DebugContext(ctx, "document read permission result", slog.Bool("can_read", canRead))
	return canRead, nil
}

// CanWriteCollection implements [port.DocumentStore].
func (s *Store) CanWriteCollection(ctx context.Context, userID model.UserID, collectionID model.CollectionID) (bool, error) {
	slog.DebugContext(ctx, "checking collection write permission",
		slog.String("user_id", string(userID)),
		slog.String("collection_id", string(collectionID)))

	var canWrite bool
	err := s.withRetry(ctx, func(ctx context.Context, db *gorm.DB) error {
		var collection Collection
		if err := db.First(&collection, "id = ? AND owner_id = ?", string(collectionID), string(userID)).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				canWrite = false
				return errors.WithStack(port.ErrNotFound)
			}
			return errors.WithStack(err)
		}
		canWrite = true
		return nil
	}, sqlite3.LOCKED, sqlite3.BUSY)

	if err != nil {
		slog.ErrorContext(ctx, "failed to check collection write permission", slog.Any("error", err))
		return false, errors.WithStack(err)
	}

	slog.DebugContext(ctx, "collection write permission result", slog.Bool("can_write", canWrite))
	return canWrite, nil
}

// CanWriteDocument implements [port.DocumentStore].
func (s *Store) CanWriteDocument(ctx context.Context, userID model.UserID, documentID model.DocumentID) (bool, error) {
	slog.DebugContext(ctx, "checking document write permission",
		slog.String("user_id", string(userID)),
		slog.String("document_id", string(documentID)))

	var canWrite bool
	err := s.withRetry(ctx, func(ctx context.Context, db *gorm.DB) error {
		var document Document
		if err := db.First(&document, "id = ? AND owner_id = ?", string(documentID), string(userID)).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				canWrite = false
				return errors.WithStack(port.ErrNotFound)
			}
			return errors.WithStack(err)
		}
		canWrite = true
		return nil
	}, sqlite3.LOCKED, sqlite3.BUSY)

	if err != nil {
		slog.ErrorContext(ctx, "failed to check document write permission", slog.Any("error", err))
		return false, errors.WithStack(err)
	}

	slog.DebugContext(ctx, "document write permission result", slog.Bool("can_write", canWrite))
	return canWrite, nil
}
