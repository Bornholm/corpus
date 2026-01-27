package gorm

import (
	"context"
	"log/slog"
	"slices"
	"time"

	"github.com/bornholm/corpus/internal/core/port"
	"github.com/ncruces/go-sqlite3"
	"github.com/pkg/errors"
	"gorm.io/gorm"
)

type Store struct {
	getDatabase func(ctx context.Context) (*gorm.DB, error)
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
		getDatabase: createGetDatabase(db,
			// Document store
			&Document{}, &Section{}, &Collection{},
			// User store
			&User{}, &AuthToken{}, &UserRole{},
			// Public share store
			&PublicShare{},
		),
	}
}

var (
	_ port.DocumentStore    = &Store{}
	_ port.UserStore        = &Store{}
	_ port.PublicShareStore = &Store{}
)
