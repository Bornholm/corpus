package model

import (
	"github.com/rs/xid"
)

type CollectionID string

func NewCollectionID() CollectionID {
	return CollectionID(xid.New().String())
}

type Collection interface {
	ID() CollectionID
	Name() string
	Description() string
}

type ReadOnlyCollection struct {
	id          CollectionID
	name        string
	description string
}

// Description implements Collection.
func (c *ReadOnlyCollection) Description() string {
	return c.description
}

// ID implements Collection.
func (c *ReadOnlyCollection) ID() CollectionID {
	return c.id
}

// Name implements Collection.
func (c *ReadOnlyCollection) Name() string {
	return c.name
}

func NewReadOnlyCollection(id CollectionID, name string, description string) *ReadOnlyCollection {
	return &ReadOnlyCollection{
		id:          id,
		name:        name,
		description: description,
	}
}

var _ Collection = &ReadOnlyCollection{}
