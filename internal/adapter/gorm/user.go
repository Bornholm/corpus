package gorm

import (
	"slices"
	"time"

	"github.com/bornholm/corpus/internal/core/model"
)

type User struct {
	ID string `gorm:"primaryKey;autoIncrement:false"`

	CreatedAt time.Time
	UpdatedAt time.Time

	Subject  string `gorm:"index"`
	Provider string `gorm:"index"`

	DisplayName string
	Email       string `gorm:"unique"`

	AuthTokens  []*AuthToken  `gorm:"foreignKey:OwnerID;constraint:OnDelete:CASCADE;"`
	Documents   []*Document   `gorm:"foreignKey:OwnerID;constraint:OnDelete:CASCADE;"`
	Collections []*Collection `gorm:"foreignKey:OwnerID;constraint:OnDelete:CASCADE;"`

	Roles []*UserRole `gorm:"constraint:OnDelete:CASCADE;"`

	Active bool
}

// fromUser converts a model.User to a GORM User
func fromUser(u model.User) *User {
	user := &User{
		ID:          string(u.ID()),
		Subject:     u.Subject(),
		Provider:    u.Provider(),
		DisplayName: u.DisplayName(),
		Email:       u.Email(),
		Active:      true, // Default to active
	}

	for _, r := range u.Roles() {
		user.Roles = append(user.Roles, &UserRole{
			User:   user,
			UserID: user.ID,
			Role:   r,
		})
	}

	return user
}

type AuthToken struct {
	ID string `gorm:"primaryKey;autoIncrement:false"`

	CreatedAt time.Time
	UpdatedAt time.Time

	Owner   *User
	OwnerID string

	Label string
	Value string `gorm:"unique"`
}

type UserRole struct {
	ID uint `gorm:"primaryKey"`

	CreatedAt time.Time

	User   *User
	UserID string `gorm:"index:user_role_index,unique"`

	Role string `gorm:"index:user_role_index,unique"`
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
	return slices.Collect(func(yield func(string) bool) {
		for _, r := range w.u.Roles {
			if !yield(r.Role) {
				return
			}
		}
	})
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

// Owner implements model.AuthToken.
func (w *wrappedAuthToken) Owner() model.User {
	return &wrappedUser{w.t.Owner}
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
