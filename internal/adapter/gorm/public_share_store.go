package gorm

import (
	"context"
	"time"

	"github.com/bornholm/corpus/internal/core/model"
	"github.com/bornholm/corpus/internal/core/port"
	"github.com/ncruces/go-sqlite3"
	"github.com/pkg/errors"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type wrappedPublicShare struct {
	ps *PublicShare
}

// CreatedAt implements [model.PersistedPublicShare].
func (w *wrappedPublicShare) CreatedAt() time.Time {
	return w.ps.CreatedAt
}

// UpdatedAt implements [model.PersistedPublicShare].
func (w *wrappedPublicShare) UpdatedAt() time.Time {
	return w.ps.UpdatedAt
}

// ID implements [model.PublicShare].
func (w *wrappedPublicShare) ID() model.PublicShareID {
	return model.PublicShareID(w.ps.ID)
}

// Token implements [model.PublicShare].
func (w *wrappedPublicShare) Token() string {
	return w.ps.Token
}

// Title implements [model.PublicShare].
func (w *wrappedPublicShare) Title() string {
	return w.ps.Title
}

// Description implements [model.PublicShare].
func (w *wrappedPublicShare) Description() string {
	return w.ps.Description
}

// OwnerID implements [model.OwnedPublicShare].
func (w *wrappedPublicShare) Owner() model.User {
	return &wrappedUser{w.ps.Owner}
}

// Collections implements [model.PublicShare].
func (w *wrappedPublicShare) Collections() []model.Collection {
	collections := make([]model.Collection, 0, len(w.ps.Collections))
	for _, c := range w.ps.Collections {
		collections = append(collections, &wrappedCollection{c})
	}
	return collections
}

var _ model.PersistedPublicShare = &wrappedPublicShare{}

func fromPublicShare(ps model.OwnedPublicShare) *PublicShare {
	publicShare := &PublicShare{
		ID:          string(ps.ID()),
		Owner:       fromUser(ps.Owner()),
		OwnerID:     string(ps.Owner().ID()),
		Token:       ps.Token(),
		Title:       ps.Title(),
		Description: ps.Description(),
	}

	// Convert collections
	collections := ps.Collections()
	publicShare.Collections = make([]*Collection, 0, len(collections))
	for _, c := range collections {
		collection := &Collection{
			ID: string(c.ID()),
		}
		publicShare.Collections = append(publicShare.Collections, collection)
	}

	return publicShare
}

// DeletePublicShare implements [port.PublicShareStore].
func (s *Store) DeletePublicShare(ctx context.Context, id model.PublicShareID) error {
	err := s.withRetry(ctx, func(ctx context.Context, db *gorm.DB) error {
		var publicShare PublicShare
		if err := db.First(&publicShare, "id = ?", string(id)).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return errors.WithStack(port.ErrNotFound)
			}
			return errors.WithStack(err)
		}

		if err := db.Select(clause.Associations).Delete(&publicShare).Error; err != nil {
			return errors.WithStack(err)
		}

		return nil
	}, sqlite3.LOCKED, sqlite3.BUSY)
	if err != nil {
		return errors.WithStack(err)
	}

	return nil
}

// FindPublicShareByToken implements [port.PublicShareStore].
func (s *Store) FindPublicShareByToken(ctx context.Context, token string) (model.PersistedPublicShare, error) {
	var publicShare PublicShare

	err := s.withRetry(ctx, func(ctx context.Context, db *gorm.DB) error {
		if err := db.Preload(clause.Associations).Preload("Collections.Owner").First(&publicShare, "token = ?", token).Error; err != nil {
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

	return &wrappedPublicShare{&publicShare}, nil
}

// GetPublicShareByID implements [port.PublicShareStore].
func (s *Store) GetPublicShareByID(ctx context.Context, id model.PublicShareID) (model.PersistedPublicShare, error) {
	var publicShare PublicShare

	err := s.withRetry(ctx, func(ctx context.Context, db *gorm.DB) error {
		if err := db.Preload(clause.Associations).Preload("Collections.Owner").First(&publicShare, "id = ?", string(id)).Error; err != nil {
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

	return &wrappedPublicShare{&publicShare}, nil
}

// QueryPublicShares implements [port.PublicShareStore].
func (s *Store) QueryPublicShares(ctx context.Context, opts port.QueryPublicSharesOptions) ([]model.PersistedPublicShare, error) {
	var publicShares []*PublicShare

	err := s.withRetry(ctx, func(ctx context.Context, db *gorm.DB) error {
		query := db.Model(&PublicShare{})

		// Apply pagination
		if opts.Page != nil {
			limit := 10
			if opts.Limit != nil {
				limit = *opts.Limit
			}
			query = query.Offset(*opts.Page * limit)
		}

		if opts.Limit != nil {
			query = query.Limit(*opts.Limit)
		}

		// Load associations unless header only
		if !opts.HeaderOnly {
			query = query.Preload(clause.Associations)
		}

		if err := query.Find(&publicShares).Error; err != nil {
			return errors.WithStack(err)
		}

		return nil
	}, sqlite3.LOCKED, sqlite3.BUSY)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	wrappedPublicShares := make([]model.PersistedPublicShare, 0, len(publicShares))
	for _, ps := range publicShares {
		wrappedPublicShares = append(wrappedPublicShares, &wrappedPublicShare{ps})
	}

	return wrappedPublicShares, nil
}

// SavePublicShare implements [port.PublicShareStore].
func (s *Store) SavePublicShare(ctx context.Context, publicShare model.OwnedPublicShare) (model.PersistedPublicShare, error) {
	var savedPublicShare *PublicShare

	err := s.withRetry(ctx, func(ctx context.Context, db *gorm.DB) error {
		ps := fromPublicShare(publicShare)

		// Check if public share already exists
		var existing PublicShare
		if err := db.First(&existing, "id = ?", ps.ID).Error; err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.WithStack(err)
		}

		if existing.ID != "" {
			// Update existing public share
			if err := db.Model(&existing).Updates(map[string]interface{}{
				"title":       ps.Title,
				"description": ps.Description,
				"token":       ps.Token,
			}).Error; err != nil {
				return errors.WithStack(err)
			}

			// Clear existing collection associations
			if err := db.Model(&existing).Association("Collections").Clear(); err != nil {
				return errors.WithStack(err)
			}

			// Add new collection associations
			if len(ps.Collections) > 0 {
				if err := db.Model(&existing).Association("Collections").Append(ps.Collections); err != nil {
					return errors.WithStack(err)
				}
			}

			savedPublicShare = &existing
		} else {
			// Create new public share
			if err := db.Model(ps).Omit("Owner.Roles").Create(ps).Error; err != nil {
				return errors.WithStack(err)
			}
			savedPublicShare = ps
		}

		// Reload with associations
		if err := db.Preload(clause.Associations).First(savedPublicShare, "id = ?", savedPublicShare.ID).Error; err != nil {
			return errors.WithStack(err)
		}

		return nil
	}, sqlite3.LOCKED, sqlite3.BUSY)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	return &wrappedPublicShare{savedPublicShare}, nil
}

var _ port.PublicShareStore = &Store{}
