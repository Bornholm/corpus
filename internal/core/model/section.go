package model

import "github.com/rs/xid"

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
	Start() int
	End() int
	Content() ([]byte, error)
}
