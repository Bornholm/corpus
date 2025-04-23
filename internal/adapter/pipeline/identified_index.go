package pipeline

import (
	"github.com/bornholm/corpus/internal/core/port"
)

type IdentifiedIndex struct {
	id    string
	index port.Index
}

func (i *IdentifiedIndex) Index() port.Index {
	return i.index
}

func (i *IdentifiedIndex) ID() string {
	return i.id
}

func NewIdentifiedIndex(id string, index port.Index) *IdentifiedIndex {
	return &IdentifiedIndex{
		id:    id,
		index: index,
	}
}
