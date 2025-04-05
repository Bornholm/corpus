package gorm

import (
	"time"

	"github.com/bornholm/corpus/internal/core/model"
)

type Collection struct {
	ID string `gorm:"primaryKey;autoIncrement:false"`

	CreatedAt time.Time
	UpdatedAt time.Time

	Name        string `gorm:"unique"`
	Label       string
	Description string

	Documents []*Document `gorm:"many2many:documents_collections;"`
}

type wrappedCollection struct {
	c *Collection
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
func (w *wrappedCollection) Name() string {
	return w.c.Name
}

var _ model.Collection = &wrappedCollection{}

func fromCollection(c model.Collection) *Collection {
	collection := &Collection{
		ID:          string(c.ID()),
		Name:        c.Name(),
		Label:       c.Label(),
		Description: c.Description(),
	}

	return collection
}
