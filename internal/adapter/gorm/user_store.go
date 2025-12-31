package gorm

import (
	"context"
	"log/slog"
	"slices"
	"time"

	"github.com/bornholm/corpus/internal/core/model"
	"github.com/bornholm/corpus/internal/core/port"
	"github.com/ncruces/go-sqlite3"
	"github.com/pkg/errors"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type UserStore struct {
	getDatabase func(ctx context.Context) (*gorm.DB, error)
}

// wrappedUser implements the model.User interface
type wrappedUser struct {
	u *User
}

// ID implements model.User.
func (w *wrappedUser) ID() model.UserID {
	return model.UserID(w.u.ID)
}

// Email implements model.User.
func (w *wrappedUser) Email() string {
	return w.u.Email
}

// Subject implements model.User.
func (w *wrappedUser) Subject() string {
	return w.u.Subject
}

// Provider implements model.User.
func (w *wrappedUser) Provider() string {
	return w.u.Provider
}

// DisplayName implements model.User.
func (w *wrappedUser) DisplayName() string {
	return w.u.DisplayName
}

// Roles implements model.User.
func (w *wrappedUser) Roles() []string {
	// For now, return empty slice as roles are not stored in the GORM model
	// This can be extended later if needed
	return []string{}
}

var _ model.User = &wrappedUser{}

// wrappedAuthToken implements the model.AuthToken interface
type wrappedAuthToken struct {
	t *AuthToken
}

// ID implements model.AuthToken.
func (w *wrappedAuthToken) ID() model.AuthTokenID {
	return model.AuthTokenID(w.t.ID)
}

// UserID implements model.AuthToken.
func (w *wrappedAuthToken) UserID() model.UserID {
	return model.UserID(w.t.UserID)
}

// Label implements model.AuthToken.
func (w *wrappedAuthToken) Label() string {
	return w.t.Label
}

// Value implements model.AuthToken.
func (w *wrappedAuthToken) Value() string {
	return w.t.Value
}

var _ model.AuthToken = &wrappedAuthToken{}

// fromUser converts a model.User to a GORM User
func fromUser(u model.User) *User {
	return &User{
		ID:          string(u.ID()),
		Subject:     u.Subject(),
		Provider:    u.Provider(),
		DisplayName: u.DisplayName(),
		Email:       u.Email(),
		Active:      true, // Default to active
	}
}

// fromAuthToken converts a model.AuthToken to a GORM AuthToken
func fromAuthToken(t model.AuthToken) *AuthToken {
	return &AuthToken{
		ID:     string(t.ID()),
		UserID: string(t.UserID()),
		Label:  t.Label(),
		Value:  t.Value(),
	}
}

// FindOrCreateUser implements port.UserStore.
func (s *UserStore) FindOrCreateUser(ctx context.Context, provider, subject string) (model.User, error) {
	var user model.User
	err := s.withRetry(ctx, func(ctx context.Context, db *gorm.DB) error {
		var u User

		err := db.Where("provider = ? AND subject = ?", provider, subject).
			Attrs(&User{
				ID:       string(model.NewUserID()),
				Provider: provider,
				Subject:  subject,
				Active:   true,
			}).
			FirstOrCreate(&u).Error
		if err != nil {
			return errors.WithStack(err)
		}

		user = &wrappedUser{&u}
		return nil
	}, sqlite3.BUSY, sqlite3.LOCKED)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	return user, nil
}

// GetUserByID implements port.UserStore.
func (s *UserStore) GetUserByID(ctx context.Context, userID model.UserID) (model.User, error) {
	var user User

	err := s.withRetry(ctx, func(ctx context.Context, db *gorm.DB) error {
		if err := db.First(&user, "id = ?", string(userID)).Error; err != nil {
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

	return &wrappedUser{&user}, nil
}

// SaveUser implements port.UserStore.
func (s *UserStore) SaveUser(ctx context.Context, user model.User) error {
	err := s.withRetry(ctx, func(ctx context.Context, db *gorm.DB) error {
		gormUser := fromUser(user)

		// Use Clauses with OnConflict to handle upsert
		if err := db.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "id"}},
			UpdateAll: true,
		}).Create(gormUser).Error; err != nil {
			return errors.WithStack(err)
		}

		return nil
	}, sqlite3.LOCKED, sqlite3.BUSY)
	if err != nil {
		return errors.WithStack(err)
	}

	return nil
}

// FindAuthToken implements port.UserStore.
func (s *UserStore) FindAuthToken(ctx context.Context, token string) (model.AuthToken, error) {
	var authToken AuthToken

	err := s.withRetry(ctx, func(ctx context.Context, db *gorm.DB) error {
		if err := db.Preload("User").First(&authToken, "value = ?", token).Error; err != nil {
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

	return &wrappedAuthToken{&authToken}, nil
}

// GetUserAuthTokens implements port.UserStore.
func (s *UserStore) GetUserAuthTokens(ctx context.Context, userID model.UserID) ([]model.AuthToken, error) {
	var authTokens []*AuthToken

	err := s.withRetry(ctx, func(ctx context.Context, db *gorm.DB) error {
		if err := db.Where("user_id = ?", string(userID)).Find(&authTokens).Error; err != nil {
			return errors.WithStack(err)
		}
		return nil
	}, sqlite3.LOCKED, sqlite3.BUSY)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	wrappedTokens := make([]model.AuthToken, 0, len(authTokens))
	for _, t := range authTokens {
		wrappedTokens = append(wrappedTokens, &wrappedAuthToken{t})
	}

	return wrappedTokens, nil
}

// CreateAuthToken implements port.UserStore.
func (s *UserStore) CreateAuthToken(ctx context.Context, token model.AuthToken) error {
	err := s.withRetry(ctx, func(ctx context.Context, db *gorm.DB) error {
		gormToken := fromAuthToken(token)

		if err := db.Create(gormToken).Error; err != nil {
			return errors.WithStack(err)
		}

		return nil
	}, sqlite3.LOCKED, sqlite3.BUSY)
	if err != nil {
		return errors.WithStack(err)
	}

	return nil
}

// DeleteAuthToken implements port.UserStore.
func (s *UserStore) DeleteAuthToken(ctx context.Context, tokenID model.AuthTokenID) error {
	err := s.withRetry(ctx, func(ctx context.Context, db *gorm.DB) error {
		result := db.Delete(&AuthToken{}, "id = ?", string(tokenID))
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

func (s *UserStore) withRetry(ctx context.Context, fn func(ctx context.Context, db *gorm.DB) error, codes ...sqlite3.ErrorCode) error {
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

func NewUserStore(db *gorm.DB) *UserStore {
	return &UserStore{
		getDatabase: createGetDatabase(db, &User{}, &AuthToken{}),
	}
}

var _ port.UserStore = &UserStore{}
