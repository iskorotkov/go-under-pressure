package cache

import (
	"github.com/dgraph-io/ristretto"
)

type URLCache struct {
	cache *ristretto.Cache
}

func New() (*URLCache, error) {
	cache, err := ristretto.NewCache(&ristretto.Config{
		NumCounters: 1e6,     // 1M counters for admission policy
		MaxCost:     1 << 27, // 128MB max
		BufferItems: 64,
	})
	if err != nil {
		return nil, err
	}
	return &URLCache{cache: cache}, nil
}

func (c *URLCache) Get(shortCode string) (string, bool) {
	val, found := c.cache.Get(shortCode)
	if !found {
		return "", false
	}
	return val.(string), true
}

func (c *URLCache) Set(shortCode, originalURL string) {
	c.cache.Set(shortCode, originalURL, 1)
}

func (c *URLCache) Close() {
	c.cache.Close()
}
