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
