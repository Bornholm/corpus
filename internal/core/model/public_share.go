package model

import (
	"github.com/rs/xid"
)

type PublicShareID string

func NewPublicShareID() PublicShareID {
	return PublicShareID(xid.New().String())
}

// Public share is an object representing a
// public share by an admin to an "ask" page
// using predefined collections.
// The public share is freely accessible to everyone
// that have the public unique link (based on the associated token)
type PublicShare interface {
	WithID[PublicShareID]

	Token() string
	Title() string
	Description() string

	Collections() []Collection
}

type OwnedPublicShare interface {
	PublicShare
	WithOwner
}

type PersistedPublicShare interface {
	OwnedPublicShare
	WithLifecycle
}
