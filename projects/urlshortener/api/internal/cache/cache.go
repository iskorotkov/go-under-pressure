package cache

import (
	"github.com/dgraph-io/ristretto"
)

type URLCache struct {
	cache *ristretto.Cache
}

func New(maxSizePow2 int) (*URLCache, error) {
	maxCost := max(1, int64(1)<<maxSizePow2)
	numCounters := max(1, maxCost/100) // ~100 bytes per entry estimate

	cache, err := ristretto.NewCache(&ristretto.Config{
		NumCounters: numCounters,
		MaxCost:     maxCost,
		BufferItems: 64,
		Metrics:     true,
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
	cost := int64(len(shortCode) + len(originalURL))
	c.cache.Set(shortCode, originalURL, cost)
}

func (c *URLCache) Close() {
	c.cache.Close()
}

func (c *URLCache) Stats() (hits, misses uint64, ratio float64) {
	metrics := c.cache.Metrics
	hits = metrics.Hits()
	misses = metrics.Misses()
	ratio = metrics.Ratio()
	return
}
