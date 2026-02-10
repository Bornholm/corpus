package cache

import "github.com/bornholm/corpus/internal/core/model"

type CacheableSection struct {
	model.Section
}

// CacheKeys implements [Cacheable].
func (d *CacheableSection) CacheKeys() []string {
	return []string{
		string(d.ID()),
	}
}

func NewCacheableSection(section model.Section) *CacheableSection {
	return &CacheableSection{section}
}

var (
	_ model.Section = &CacheableSection{}
	_ Cacheable     = &CacheableSection{}
)
