package gorm

import (
	"context"
	"log/slog"
	"net/url"
	"slices"
	"sync"
	"time"

	"github.com/bornholm/corpus/internal/core/model"
	"github.com/bornholm/corpus/internal/core/port"
	"github.com/ncruces/go-sqlite3"
	"github.com/pkg/errors"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Store struct {
	getDatabase func(ctx context.Context) (*gorm.DB, error)
}

// SectionExists implements port.Store.
func (s *Store) SectionExists(ctx context.Context, id model.SectionID) (bool, error) {
	var exists bool

	err := s.withRetry(ctx, func(ctx context.Context, db *gorm.DB) error {
		var count int64
		if err := db.Model(&Section{}).Where("id = ?", string(id)).Count(&count).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return errors.WithStack(port.ErrNotFound)
			}

			return errors.WithStack(port.ErrNotFound)
		}

		exists = count > 0

		return nil
	}, sqlite3.LOCKED, sqlite3.BUSY)
	if err != nil {
		return false, errors.WithStack(err)
	}

	return exists, nil
}

// GetDocumentByID implements port.Store.
func (s *Store) GetDocumentByID(ctx context.Context, id model.DocumentID) (model.Document, error) {
	var document Document

	err := s.withRetry(ctx, func(ctx context.Context, db *gorm.DB) error {
		if err := db.Preload(clause.Associations).Preload("Sections", preloadSections).First(&document, "id = ?", id).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return errors.WithStack(port.ErrNotFound)
			}

			return errors.WithStack(port.ErrNotFound)
		}

		return nil
	}, sqlite3.LOCKED, sqlite3.BUSY)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	return &wrappedDocument{&document}, nil
}

// GetCollectionStats implements port.Store.
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

// CreateCollection implements port.Store.
func (s *Store) CreateCollection(ctx context.Context, name string) (model.Collection, error) {
	db, err := s.getDatabase(ctx)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	collection := &Collection{
		ID:   string(model.NewCollectionID()),
		Name: name,
	}

	if err := db.Create(collection).Error; err != nil {
		return nil, errors.WithStack(err)
	}

	return &wrappedCollection{collection}, nil
}

// UpdateCollection implements port.Store.
func (s *Store) UpdateCollection(ctx context.Context, id model.CollectionID, updates port.CollectionUpdates) (model.Collection, error) {
	panic("unimplemented")
}

// GetCollectionByName implements port.Store.
func (s *Store) GetCollectionByName(ctx context.Context, name string) (model.Collection, error) {
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

// QueryCollections implements port.Store.
func (s *Store) QueryCollections(ctx context.Context, opts port.QueryCollectionsOptions) ([]model.Collection, error) {
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

// CountDocuments implements port.Store.
func (s *Store) CountDocuments(ctx context.Context) (int64, error) {
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

// GetSectionByID implements port.Store.
func (s *Store) GetSectionByID(ctx context.Context, id model.SectionID) (model.Section, error) {
	var section Section

	err := s.withRetry(ctx, func(ctx context.Context, db *gorm.DB) error {
		if err := db.Preload(clause.Associations, preloadSections).First(&section, "id = ?", id).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return errors.WithStack(port.ErrNotFound)
			}

			return errors.WithStack(port.ErrNotFound)
		}

		return nil
	}, sqlite3.LOCKED, sqlite3.BUSY)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	return &wrappedSection{&section}, nil
}

// DeleteDocumentBySource implements port.Store.
func (s *Store) DeleteDocumentBySource(ctx context.Context, source *url.URL) error {
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

		if err := db.Select(clause.Associations).Preload("Sections", preloadSections).Delete(&doc).Error; err != nil {
			return errors.WithStack(err)
		}

		return nil
	}, sqlite3.LOCKED, sqlite3.BUSY)
	if err != nil {
		return errors.WithStack(err)
	}

	return nil
}

// QueryDocuments implements port.Store.
func (s *Store) QueryDocuments(ctx context.Context, opts port.QueryDocumentsOptions) ([]model.Document, int64, error) {
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
			query = query.Preload(clause.Associations).Preload("Sections", preloadSections)
		} else {
			query = query.Select("ID", "CreatedAt", "UpdatedAt", "Source")
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

func preloadSections(db *gorm.DB) *gorm.DB {
	return db.Preload("Sections", preloadSections)
}

// SaveDocuments implements port.Store.
func (s *Store) SaveDocuments(ctx context.Context, documents ...model.Document) error {
	err := s.withRetry(ctx, func(ctx context.Context, db *gorm.DB) error {
		for _, doc := range documents {
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

			if res := db.Create(document); res.Error != nil {
				return errors.WithStack(res.Error)
			}
		}

		return nil
	}, sqlite3.LOCKED, sqlite3.BUSY)
	if err != nil {
		return errors.WithStack(err)
	}

	return nil
}

func (s *Store) withRetry(ctx context.Context, fn func(ctx context.Context, db *gorm.DB) error, codes ...sqlite3.ErrorCode) error {
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

func NewStore(db *gorm.DB) *Store {
	return &Store{
		getDatabase: createGetDatabase(db),
	}
}

var _ port.Store = &Store{}

func createGetDatabase(db *gorm.DB) func(ctx context.Context) (*gorm.DB, error) {
	var (
		migrateOnce sync.Once
		migrateErr  error
	)

	return func(ctx context.Context) (*gorm.DB, error) {
		migrateOnce.Do(func() {
			models := []any{
				&Document{},
				&Section{},
			}

			if err := db.AutoMigrate(models...); err != nil {
				migrateErr = errors.WithStack(err)
				return
			}
		})
		if migrateErr != nil {
			return nil, errors.WithStack(migrateErr)
		}

		return db, nil
	}
}
