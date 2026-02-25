package model

import (
	"github.com/rs/xid"
)

type CollectionShareID string

func NewCollectionShareID() CollectionShareID {
	return CollectionShareID(xid.New().String())
}

type CollectionShareLevel string

const (
	CollectionShareLevelRead  CollectionShareLevel = "read"
	CollectionShareLevelWrite CollectionShareLevel = "write"
)

// CollectionShare represents a share of a collection with a specific user at a given access level.
type CollectionShare interface {
	WithID[CollectionShareID]

	CollectionID() CollectionID
	SharedWith() User
	Level() CollectionShareLevel
}

// PersistedCollectionShare is a CollectionShare that has been persisted to the store.
type PersistedCollectionShare interface {
	CollectionShare
	WithLifecycle
}

type BaseCollectionShare struct {
	id           CollectionShareID
	collectionID CollectionID
	sharedWith   User
	level        CollectionShareLevel
}

// ID implements CollectionShare.
func (s *BaseCollectionShare) ID() CollectionShareID {
	return s.id
}

// CollectionID implements CollectionShare.
func (s *BaseCollectionShare) CollectionID() CollectionID {
	return s.collectionID
}

// SharedWith implements CollectionShare.
func (s *BaseCollectionShare) SharedWith() User {
	return s.sharedWith
}

// Level implements CollectionShare.
func (s *BaseCollectionShare) Level() CollectionShareLevel {
	return s.level
}

func NewCollectionShare(collectionID CollectionID, sharedWith User, level CollectionShareLevel) *BaseCollectionShare {
	return &BaseCollectionShare{
		id:           NewCollectionShareID(),
		collectionID: collectionID,
		sharedWith:   sharedWith,
		level:        level,
	}
}

var _ CollectionShare = &BaseCollectionShare{}
