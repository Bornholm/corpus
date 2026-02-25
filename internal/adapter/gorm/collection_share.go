package gorm

import (
	"time"

	"github.com/bornholm/corpus/internal/core/model"
)

// CollectionShare is the GORM model for collection shares.
type CollectionShare struct {
	ID string `gorm:"primaryKey;autoIncrement:false"`

	CreatedAt time.Time
	UpdatedAt time.Time

	CollectionID string
	Collection   *Collection

	UserID string
	User   *User

	Level string
}

type wrappedCollectionShare struct {
	cs *CollectionShare
}

// CreatedAt implements [model.PersistedCollectionShare].
func (w *wrappedCollectionShare) CreatedAt() time.Time {
	return w.cs.CreatedAt
}

// UpdatedAt implements [model.PersistedCollectionShare].
func (w *wrappedCollectionShare) UpdatedAt() time.Time {
	return w.cs.UpdatedAt
}

// ID implements [model.CollectionShare].
func (w *wrappedCollectionShare) ID() model.CollectionShareID {
	return model.CollectionShareID(w.cs.ID)
}

// CollectionID implements [model.CollectionShare].
func (w *wrappedCollectionShare) CollectionID() model.CollectionID {
	return model.CollectionID(w.cs.CollectionID)
}

// SharedWith implements [model.CollectionShare].
func (w *wrappedCollectionShare) SharedWith() model.User {
	return &wrappedUser{w.cs.User}
}

// Level implements [model.CollectionShare].
func (w *wrappedCollectionShare) Level() model.CollectionShareLevel {
	return model.CollectionShareLevel(w.cs.Level)
}

var _ model.PersistedCollectionShare = &wrappedCollectionShare{}
