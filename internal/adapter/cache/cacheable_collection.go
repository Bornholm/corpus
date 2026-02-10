package cache

import (
	"github.com/bornholm/corpus/internal/core/model"
)

type CacheableCollection struct {
	model.PersistedCollection
}

// CacheKeys implements [Cacheable].
func (c *CacheableCollection) CacheKeys() []string {
	return []string{
		string(c.ID()),
	}
}

func NewCacheableCollection(collection model.PersistedCollection) *CacheableCollection {
	return &CacheableCollection{collection}
}

var (
	_ model.PersistedCollection = &CacheableCollection{}
	_ Cacheable                 = &CacheableCollection{}
)
