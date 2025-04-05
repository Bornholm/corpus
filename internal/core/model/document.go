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
	ID() DocumentID
	Source() *url.URL
	Collections() []Collection
	Sections() []Section
	Content() ([]byte, error)
	Chunk(start, end int) ([]byte, error)
}
