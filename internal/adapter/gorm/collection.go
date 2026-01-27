package gorm

import (
	"time"

	"github.com/bornholm/corpus/internal/core/model"
)

type Collection struct {
	ID string `gorm:"primaryKey;autoIncrement:false"`

	CreatedAt time.Time
	UpdatedAt time.Time

	Owner   *User
	OwnerID string

	Label       string
	Description string

	Documents []*Document `gorm:"many2many:documents_collections;constraint:OnDelete:CASCADE"`

	PublicShares []*PublicShare `gorm:"many2many:public_shares_collections;"`
}

type wrappedCollection struct {
	c *Collection
}

// CreatedAt implements [model.PersistedCollection].
func (w *wrappedCollection) CreatedAt() time.Time {
	return w.c.CreatedAt
}

// UpdatedAt implements [model.PersistedCollection].
func (w *wrappedCollection) UpdatedAt() time.Time {
	return w.c.UpdatedAt
}

// Description implements model.Collection.
func (w *wrappedCollection) Description() string {
	return w.c.Description
}

// ID implements model.Collection.
func (w *wrappedCollection) ID() model.CollectionID {
	return model.CollectionID(w.c.ID)
}

// LAbel implements model.Collection.
func (w *wrappedCollection) Label() string {
	return w.c.Label
}

// Name implements model.Collection.
func (w *wrappedCollection) OwnerID() model.UserID {
	return model.UserID(w.c.OwnerID)
}

var _ model.PersistedCollection = &wrappedCollection{}

func fromCollection(c model.OwnedCollection) *Collection {
	collection := &Collection{
		ID:          string(c.ID()),
		OwnerID:     string(c.OwnerID()),
		Label:       c.Label(),
		Description: c.Description(),
	}

	return collection
}
