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

	Name() string
	Label() string
	Description() string
}

type CollectionStats struct {
	TotalDocuments int64
}
type ReadOnlyCollection struct {
	id          CollectionID
	name        string
	label       string
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

// Label implements Collection.
func (c *ReadOnlyCollection) Label() string {
	return c.label
}

func NewReadOnlyCollection(id CollectionID, name string, label string, description string) *ReadOnlyCollection {
	return &ReadOnlyCollection{
		id:          id,
		name:        name,
		label:       label,
		description: description,
	}
}

var _ Collection = &ReadOnlyCollection{}
