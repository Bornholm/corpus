package model

import (
	"github.com/rs/xid"
)

type CollectionID string

func NewCollectionID() CollectionID {
	return CollectionID(xid.New().String())
}

type Collection interface {
	WithID[CollectionID]

	Label() string
	Description() string
}

type OwnedCollection interface {
	Collection

	WithOwner
}

type PersistedCollection interface {
	OwnedCollection

	WithLifecycle
}

type CollectionStats struct {
	TotalDocuments int64
}
type BaseCollection struct {
	id          CollectionID
	label       string
	description string
	ownerID     UserID
}

// Description implements Collection.
func (c *BaseCollection) Description() string {
	return c.description
}

// ID implements Collection.
func (c *BaseCollection) ID() CollectionID {
	return c.id
}

// Name implements Collection.
func (c *BaseCollection) OwnerID() UserID {
	return c.ownerID
}

// Label implements Collection.
func (c *BaseCollection) Label() string {
	return c.label
}

func NewCollection(id CollectionID, ownerID UserID, label string, description string) *BaseCollection {
	return &BaseCollection{
		id:          id,
		ownerID:     ownerID,
		label:       label,
		description: description,
	}
}

var _ Collection = &BaseCollection{}
