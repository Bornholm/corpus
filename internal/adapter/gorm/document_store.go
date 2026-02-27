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
	err := s.withRetry(ctx, true, func(ctx context.Context, db *gorm.DB) error {
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

	err := s.withRetry(ctx, false, func(ctx context.Context, db *gorm.DB) error {
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

	err := s.withRetry(ctx, false, func(ctx context.Context, db *gorm.DB) error {
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

	err := s.withRetry(ctx, false, func(ctx context.Context, db *gorm.DB) error {
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

	stats := &model.CollectionStats{
		TotalDocuments: db.Model(&Collection{ID: string(id)}).Association("Documents").Count(),
	}

	return stats, nil
}

// CreateCollection implements port.DocumentStore.
func (s *Store) CreateCollection(ctx context.Context, ownerID model.UserID, label string) (model.PersistedCollection, error) {
	var collection model.PersistedCollection
	err := s.withRetry(ctx, true, func(ctx context.Context, db *gorm.DB) error {
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

	err := s.withRetry(ctx, true, func(ctx context.Context, db *gorm.DB) error {
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
	err := s.withRetry(ctx, true, func(ctx context.Context, db *gorm.DB) error {

		if err := db.Model(&Collection{
			ID: string(id),
		}).Association("PublicShares").Clear(); err != nil {
			return errors.WithStack(err)
		}

		// Explicitly delete collection shares (in case FK cascade is not enforced)
		if err := db.Where("collection_id = ?", string(id)).Delete(&CollectionShare{}).Error; err != nil {
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

	// Count documents the user owns OR documents in collections shared with the user
	if err := db.Model(&Document{}).Where(
		"owner_id = ? OR id IN (SELECT document_id FROM documents_collections WHERE collection_id IN (SELECT collection_id FROM collection_shares WHERE user_id = ?))",
		string(userID), string(userID),
	).Count(&total).Error; err != nil {
		return 0, errors.WithStack(err)
	}

	return total, nil
}

// GetSectionByID implements port.DocumentStore.
func (s *Store) GetSectionByID(ctx context.Context, id model.SectionID) (model.Section, error) {
	var section Section

	err := s.withRetry(ctx, false, func(ctx context.Context, db *gorm.DB) error {
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

// GetSectionsByIDs implements port.DocumentStore.
func (s *Store) GetSectionsByIDs(ctx context.Context, ids []model.SectionID) (map[model.SectionID]model.Section, error) {
	if len(ids) == 0 {
		return make(map[model.SectionID]model.Section), nil
	}

	var sections []Section

	rawIDs := make([]string, len(ids))
	for i, id := range ids {
		rawIDs[i] = string(id)
	}

	err := s.withRetry(ctx, false, func(ctx context.Context, db *gorm.DB) error {
		if err := db.Preload(clause.Associations).Find(&sections, "id in ?", rawIDs).Error; err != nil {
			return errors.WithStack(err)
		}

		return nil
	}, sqlite3.LOCKED, sqlite3.BUSY)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	result := make(map[model.SectionID]model.Section, len(sections))
	for i := range sections {
		result[model.SectionID(sections[i].ID)] = &wrappedSection{&sections[i]}
	}

	return result, nil
}

// DeleteDocumentBySource implements port.DocumentStore.
func (s *Store) DeleteDocumentBySource(ctx context.Context, ownerID model.UserID, source *url.URL) error {
	if source == nil {
		return errors.WithStack(ErrMissingSource)
	}

	err := s.withRetry(ctx, true, func(ctx context.Context, db *gorm.DB) error {
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

	err := s.withRetry(ctx, false, func(ctx context.Context, db *gorm.DB) error {
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

	err := s.withRetry(ctx, false, func(ctx context.Context, db *gorm.DB) error {
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

	err := s.withRetry(ctx, false, func(ctx context.Context, db *gorm.DB) error {
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
		err := s.withRetry(ctx, true, func(ctx context.Context, db *gorm.DB) error {
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

			if res := db.Omit("Sections", "Owner.Roles").Create(document); res.Error != nil {
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

	err := s.withRetry(ctx, false, func(ctx context.Context, db *gorm.DB) error {
		// User can write to collections they own OR collections shared with them at write level
		query := db.Model(&Collection{}).Where(
			"owner_id = ? OR id IN (SELECT collection_id FROM collection_shares WHERE user_id = ? AND level = ?)",
			string(userID), string(userID), string(model.CollectionShareLevelWrite),
		)

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
		if err := query.Preload("Owner").Find(&collections).Error; err != nil {
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

	err := s.withRetry(ctx, false, func(ctx context.Context, db *gorm.DB) error {
		// Users can read collections they own OR collections shared with them at any level
		query := db.Model(&Collection{}).Where(
			"owner_id = ? OR id IN (SELECT collection_id FROM collection_shares WHERE user_id = ?)",
			string(userID), string(userID),
		)

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
		if err := query.Preload("Owner").Find(&collections).Error; err != nil {
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
	err := s.withRetry(ctx, false, func(ctx context.Context, db *gorm.DB) error {
		var count int64
		// User can read if they own the collection OR have any share level
		err := db.Model(&Collection{}).Where(
			"(id = ? AND owner_id = ?) OR (id = ? AND id IN (SELECT collection_id FROM collection_shares WHERE user_id = ?))",
			string(collectionID), string(userID),
			string(collectionID), string(userID),
		).Count(&count).Error
		if err != nil {
			return errors.WithStack(err)
		}
		if count == 0 {
			return errors.WithStack(port.ErrNotFound)
		}
		canRead = count > 0
		return nil
	}, sqlite3.LOCKED, sqlite3.BUSY)

	if err != nil {
		if errors.Is(err, port.ErrNotFound) {
			return false, errors.WithStack(err)
		}
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
	err := s.withRetry(ctx, false, func(ctx context.Context, db *gorm.DB) error {
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
	err := s.withRetry(ctx, false, func(ctx context.Context, db *gorm.DB) error {
		var count int64
		// User can write if they own the collection OR have a write-level share
		err := db.Model(&Collection{}).Where(
			"(id = ? AND owner_id = ?) OR (id = ? AND id IN (SELECT collection_id FROM collection_shares WHERE user_id = ? AND level = ?))",
			string(collectionID), string(userID),
			string(collectionID), string(userID), string(model.CollectionShareLevelWrite),
		).Count(&count).Error
		if err != nil {
			return errors.WithStack(err)
		}
		if count == 0 {
			return errors.WithStack(port.ErrNotFound)
		}
		canWrite = count > 0
		return nil
	}, sqlite3.LOCKED, sqlite3.BUSY)

	if err != nil {
		if errors.Is(err, port.ErrNotFound) {
			return false, errors.WithStack(err)
		}
		slog.ErrorContext(ctx, "failed to check collection write permission", slog.Any("error", err))
		return false, errors.WithStack(err)
	}

	slog.DebugContext(ctx, "collection write permission result", slog.Bool("can_write", canWrite))
	return canWrite, nil
}

// CreateCollectionShare implements [port.DocumentStore].
func (s *Store) CreateCollectionShare(ctx context.Context, collectionID model.CollectionID, userID model.UserID, level model.CollectionShareLevel) (model.PersistedCollectionShare, error) {
	var share CollectionShare

	err := s.withRetry(ctx, true, func(ctx context.Context, db *gorm.DB) error {
		// Check if a share already exists for this collection+user pair
		var existing CollectionShare
		err := db.Where("collection_id = ? AND user_id = ?", string(collectionID), string(userID)).
			First(&existing).Error

		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.WithStack(err)
		}

		if existing.ID != "" {
			// Update existing share level
			if err := db.Model(&existing).Update("level", string(level)).Error; err != nil {
				return errors.WithStack(err)
			}
			share = existing
		} else {
			// Create new share
			newShare := CollectionShare{
				ID:           string(model.NewCollectionShareID()),
				CollectionID: string(collectionID),
				UserID:       string(userID),
				Level:        string(level),
			}
			if err := db.Create(&newShare).Error; err != nil {
				return errors.WithStack(err)
			}
			share = newShare
		}

		// Reload with user association
		if err := db.Preload("User").First(&share, "id = ?", share.ID).Error; err != nil {
			return errors.WithStack(err)
		}

		return nil
	}, sqlite3.LOCKED, sqlite3.BUSY)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	return &wrappedCollectionShare{&share}, nil
}

// DeleteCollectionShare implements [port.DocumentStore].
func (s *Store) DeleteCollectionShare(ctx context.Context, shareID model.CollectionShareID) error {
	err := s.withRetry(ctx, true, func(ctx context.Context, db *gorm.DB) error {
		result := db.Delete(&CollectionShare{}, "id = ?", string(shareID))
		if result.Error != nil {
			return errors.WithStack(result.Error)
		}
		if result.RowsAffected == 0 {
			return errors.WithStack(port.ErrNotFound)
		}
		return nil
	}, sqlite3.LOCKED, sqlite3.BUSY)
	if err != nil {
		return errors.WithStack(err)
	}
	return nil
}

// GetCollectionShares implements [port.DocumentStore].
func (s *Store) GetCollectionShares(ctx context.Context, collectionID model.CollectionID) ([]model.PersistedCollectionShare, error) {
	var shares []*CollectionShare

	err := s.withRetry(ctx, false, func(ctx context.Context, db *gorm.DB) error {
		if err := db.Preload("User").Where("collection_id = ?", string(collectionID)).Find(&shares).Error; err != nil {
			return errors.WithStack(err)
		}
		return nil
	}, sqlite3.LOCKED, sqlite3.BUSY)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	result := make([]model.PersistedCollectionShare, 0, len(shares))
	for _, s := range shares {
		result = append(result, &wrappedCollectionShare{s})
	}
	return result, nil
}

// CanWriteDocument implements [port.DocumentStore].
func (s *Store) CanWriteDocument(ctx context.Context, userID model.UserID, documentID model.DocumentID) (bool, error) {
	slog.DebugContext(ctx, "checking document write permission",
		slog.String("user_id", string(userID)),
		slog.String("document_id", string(documentID)))

	var canWrite bool
	err := s.withRetry(ctx, false, func(ctx context.Context, db *gorm.DB) error {
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

// QueryDocumentsByCollectionID implements port.DocumentStore.
func (s *Store) QueryDocumentsByCollectionID(ctx context.Context, collectionID model.CollectionID, opts port.QueryDocumentsOptions) ([]model.PersistedDocument, int64, error) {
	var (
		documents []*Document
		total     int64
	)

	err := s.withRetry(ctx, false, func(ctx context.Context, db *gorm.DB) error {
		limit := 10
		if opts.Limit != nil {
			limit = *opts.Limit
		}

		page := 0
		if opts.Page != nil {
			page = *opts.Page
		}

		// Query documents that belong to the specified collection
		// using the many-to-many join table
		query := db.Model(&Document{}).Where(
			"id IN (SELECT document_id FROM documents_collections WHERE collection_id = ?)",
			string(collectionID),
		)

		if err := query.Count(&total).Error; err != nil {
			return errors.WithStack(err)
		}

		query = query.Limit(limit).Offset(page * limit).Order("created_at DESC")

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
