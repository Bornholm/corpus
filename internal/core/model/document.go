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
	Collection() string
	Sections() []Section
}

type SectionID string

func NewSectionID() SectionID {
	return SectionID(xid.New().String())
}

type Section interface {
	ID() SectionID
	Branch() []SectionID
	Level() uint
	Document() Document
	Parent() Section
	Sections() []Section
	Content() string
}
