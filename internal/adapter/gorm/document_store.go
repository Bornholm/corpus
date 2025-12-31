package gorm

import (
	"context"
	"log/slog"
	"net/url"
	"slices"
	"time"

	"github.com/bornholm/corpus/internal/core/model"
	"github.com/bornholm/corpus/internal/core/port"
	"github.com/ncruces/go-sqlite3"
	"github.com/pkg/errors"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type DocumentStore struct {
	getDatabase func(ctx context.Context) (*gorm.DB, error)
}

// DeleteDocumentByID implements port.DocumentStore.
func (s *DocumentStore) DeleteDocumentByID(ctx context.Context, id model.DocumentID) error {
	err := s.withRetry(ctx, func(ctx context.Context, db *gorm.DB) error {
		var doc Document
		if err := db.First(&doc, "id = ?", id).Error; err != nil {
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

// SectionExists implements port.DocumentStore.
func (s *DocumentStore) SectionExists(ctx context.Context, id model.SectionID) (bool, error) {
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
func (s *DocumentStore) GetDocumentByID(ctx context.Context, id model.DocumentID) (model.Document, error) {
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

// GetCollectionStats implements port.DocumentStore.
func (s *DocumentStore) GetCollectionStats(ctx context.Context, id model.CollectionID) (*model.CollectionStats, error) {
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
func (s *DocumentStore) CreateCollection(ctx context.Context, name string) (model.Collection, error) {
	var collection model.Collection
	err := s.withRetry(ctx, func(ctx context.Context, db *gorm.DB) error {
		var coll Collection

		err := db.Where("name = ?", name).
			Attrs(&Collection{ID: string(model.NewCollectionID()), Name: name}).
			FirstOrCreate(&coll).Error
		if err != nil {
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
func (s *DocumentStore) UpdateCollection(ctx context.Context, id model.CollectionID, updates port.CollectionUpdates) (model.Collection, error) {
	panic("unimplemented")
}

// GetCollectionByName implements port.DocumentStore.
func (s *DocumentStore) GetCollectionByName(ctx context.Context, name string) (model.Collection, error) {
	db, err := s.getDatabase(ctx)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	var collection Collection

	if err := db.Where("name = ?", name).First(&collection).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.WithStack(port.ErrNotFound)
		}

		return nil, errors.WithStack(err)
	}

	return &wrappedCollection{&collection}, nil
}

// QueryCollections implements port.DocumentStore.
func (s *DocumentStore) QueryCollections(ctx context.Context, opts port.QueryCollectionsOptions) ([]model.Collection, error) {
	db, err := s.getDatabase(ctx)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	var collections []*Collection

	if err := db.Find(&collections).Error; err != nil {
		return nil, errors.WithStack(err)
	}

	wrappedCollections := make([]model.Collection, 0, len(collections))
	for _, c := range collections {
		wrappedCollections = append(wrappedCollections, &wrappedCollection{c})
	}

	return wrappedCollections, nil
}

// CountDocuments implements port.DocumentStore.
func (s *DocumentStore) CountDocuments(ctx context.Context) (int64, error) {
	db, err := s.getDatabase(ctx)
	if err != nil {
		return 0, errors.WithStack(err)
	}

	var total int64

	if err := db.Model(&Document{}).Count(&total).Error; err != nil {
		return 0, errors.WithStack(err)
	}

	return total, nil
}

// GetSectionByID implements port.DocumentStore.
func (s *DocumentStore) GetSectionByID(ctx context.Context, id model.SectionID) (model.Section, error) {
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
func (s *DocumentStore) DeleteDocumentBySource(ctx context.Context, source *url.URL) error {
	if source == nil {
		return errors.WithStack(ErrMissingSource)
	}

	err := s.withRetry(ctx, func(ctx context.Context, db *gorm.DB) error {
		var doc Document
		if err := db.First(&doc, "source = ?", source.String()).Error; err != nil {
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
func (s *DocumentStore) QueryDocuments(ctx context.Context, opts port.QueryDocumentsOptions) ([]model.Document, int64, error) {
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

		if err := query.Find(&documents).Error; err != nil {
			return errors.WithStack(err)
		}

		return nil
	}, sqlite3.BUSY, sqlite3.LOCKED)
	if err != nil {
		return nil, total, errors.WithStack(err)
	}

	wrappedDocuments := make([]model.Document, 0, len(documents))
	for _, d := range documents {
		wrappedDocuments = append(wrappedDocuments, &wrappedDocument{d})
	}

	return wrappedDocuments, total, nil
}

// SaveDocuments implements port.DocumentStore.
func (s *DocumentStore) SaveDocuments(ctx context.Context, documents ...model.Document) error {
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
						Columns:   []clause.Column{{Name: "id"}, {Name: "document_id"}},
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
							"parent_id":          s.ID,
							"document_id":        s.DocumentID,
							"parent_document_id": s.DocumentID,
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

func (s *DocumentStore) withRetry(ctx context.Context, fn func(ctx context.Context, db *gorm.DB) error, codes ...sqlite3.ErrorCode) error {
	db, err := s.getDatabase(ctx)
	if err != nil {
		return errors.WithStack(err)
	}

	backoff := 500 * time.Millisecond
	maxRetries := 10
	retries := 0

	for {
		err := db.Transaction(func(tx *gorm.DB) error {
			if err := fn(ctx, tx); err != nil {
				return errors.WithStack(err)
			}

			return nil
		})
		if err != nil {
			if retries >= maxRetries {
				return errors.WithStack(err)
			}

			var sqliteErr *sqlite3.Error
			if errors.As(err, &sqliteErr) {
				if !slices.Contains(codes, sqliteErr.Code()) {
					return errors.WithStack(err)
				}

				slog.DebugContext(ctx, "transaction failed, will retry", slog.Int("retries", retries), slog.Duration("backoff", backoff), slog.Any("error", errors.WithStack(err)))

				retries++
				time.Sleep(backoff)
				backoff *= 2
				continue
			}

			return errors.WithStack(err)
		}

		return nil
	}
}

func NewDocumentStore(db *gorm.DB) *DocumentStore {
	return &DocumentStore{
		getDatabase: createGetDatabase(db, &Document{}, &Section{}),
	}
}

var _ port.DocumentStore = &DocumentStore{}
