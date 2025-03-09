package adapter

import (
	"context"
	"net/url"
	"sync"

	"github.com/bornholm/corpus/internal/core/model"
	"github.com/bornholm/corpus/internal/core/port"
	"github.com/pkg/errors"
	"gorm.io/gorm"
)

type Store struct {
	getDatabase func(ctx context.Context) (*gorm.DB, error)
}

// GetSectionBySourceAndID implements port.Store.
func (s *Store) GetSectionBySourceAndID(ctx context.Context, source *url.URL, id model.SectionID) (model.Section, error) {
	db, err := s.getDatabase(ctx)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	var section Section

	err = db.Preload("Document").
		Joins("left join documents on documents.id = sections.document_id").
		Where("sections.id = ? and documents.source = ?", string(id), source.String()).
		First(&section).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.WithStack(port.ErrNotFound)
		}

		return nil, errors.WithStack(err)
	}

	return &wrappedSection{&section}, nil
}

// DeleteDocumentBySource implements port.Store.
func (s *Store) DeleteDocumentBySource(ctx context.Context, source *url.URL) error {
	db, err := s.getDatabase(ctx)
	if err != nil {
		return errors.WithStack(err)
	}

	if err := db.Delete(&Document{}, "source = ?", source.String()).Error; err != nil {
		return errors.WithStack(err)
	}

	return nil
}

// GetDocumentBySource implements port.Store.
func (s *Store) GetDocumentBySource(ctx context.Context, source *url.URL) (model.Document, error) {
	panic("unimplemented")
}

// QueryDocuments implements port.Store.
func (s *Store) QueryDocuments(ctx context.Context, opts port.QueryDocumentsOptions) ([]*model.Document, int64, error) {
	panic("unimplemented")
}

// SaveDocument implements port.Store.
func (s *Store) SaveDocument(ctx context.Context, doc model.Document) error {
	db, err := s.getDatabase(ctx)
	if err != nil {
		return errors.WithStack(err)
	}

	err = db.Transaction(func(tx *gorm.DB) error {
		if res := tx.Delete(&Document{}, "source = ?", doc.Source().String()); res.Error != nil {
			return errors.WithStack(res.Error)
		}

		document := fromDocument(doc)

		if res := tx.Create(document); res.Error != nil {
			return errors.WithStack(res.Error)
		}

		return nil
	})
	if err != nil {
		return errors.WithStack(err)
	}

	return nil
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
