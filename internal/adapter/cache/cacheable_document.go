package cache

import (
	"github.com/bornholm/corpus/internal/core/model"
)

type CacheableDocument struct {
	model.PersistedDocument
}

// CacheKeys implements [Cacheable].
func (d *CacheableDocument) CacheKeys() []string {
	return []string{
		string(d.ID()),
		getCompositeCacheKey(d.Owner().ID(), d.Source()),
	}
}

func NewCacheableDocument(document model.PersistedDocument) *CacheableDocument {
	return &CacheableDocument{document}
}

var (
	_ model.PersistedDocument = &CacheableDocument{}
	_ Cacheable               = &CacheableDocument{}
)
