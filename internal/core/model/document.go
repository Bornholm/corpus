package model

import (
	"net/url"

	"github.com/rs/xid"
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
}
