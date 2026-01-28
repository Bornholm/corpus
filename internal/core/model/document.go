package model

import (
	"errors"
	"net/url"

	"github.com/rs/xid"
)

var (
	ErrOutOfRange = errors.New("out of range")
)

type DocumentID string

func NewDocumentID() DocumentID {
	return DocumentID(xid.New().String())
}

type Document interface {
	WithID[DocumentID]

	Source() *url.URL
	ETag() string
	Collections() []Collection
	Sections() []Section
	Content() ([]byte, error)
	Chunk(start, end int) ([]byte, error)
}

type OwnedDocument interface {
	Document
	WithOwner
}

type ownedDocument struct {
	Document
	owner User
}

func (d *ownedDocument) Owner() User {
	return d.owner
}

func AsOwnedDocument(doc Document, owner User) OwnedDocument {
	return &ownedDocument{
		Document: doc,
		owner:    owner,
	}
}

type PersistedDocument interface {
	OwnedDocument
	WithLifecycle
}
