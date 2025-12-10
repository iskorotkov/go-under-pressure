package cache_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"urlshortener/internal/cache"
)

func TestNew_ValidSize(t *testing.T) {
	c, err := cache.New(10) // 2^10 = 1KB
	require.NoError(t, err)
	require.NotNil(t, c)
	defer c.Close()
}

func TestNew_ZeroSize(t *testing.T) {
	c, err := cache.New(0) // 2^0 = 1 byte (min)
	require.NoError(t, err)
	require.NotNil(t, c)
	defer c.Close()
}

func TestGet_MissingKey(t *testing.T) {
	c, err := cache.New(10)
	require.NoError(t, err)
	defer c.Close()

	val, found := c.Get("nonexistent")
	assert.False(t, found)
	assert.Empty(t, val)
}

func TestSetThenGet(t *testing.T) {
	c, err := cache.New(20) // 2^20 = 1MB
	require.NoError(t, err)
	defer c.Close()

	shortCode := "abc123"
	originalURL := "https://example.com/very/long/path"

	c.Set(shortCode, originalURL)
	time.Sleep(10 * time.Millisecond) // Ristretto needs time to process

	val, found := c.Get(shortCode)
	assert.True(t, found)
	assert.Equal(t, originalURL, val)
}

func TestSet_UpdateExisting(t *testing.T) {
	c, err := cache.New(20)
	require.NoError(t, err)
	defer c.Close()

	shortCode := "abc123"
	url1 := "https://example.com/first"
	url2 := "https://example.com/second"

	c.Set(shortCode, url1)
	time.Sleep(10 * time.Millisecond)

	c.Set(shortCode, url2)
	time.Sleep(10 * time.Millisecond)

	val, found := c.Get(shortCode)
	assert.True(t, found)
	assert.Equal(t, url2, val)
}

func TestSet_MultipleKeys(t *testing.T) {
	c, err := cache.New(20)
	require.NoError(t, err)
	defer c.Close()

	entries := map[string]string{
		"code1": "https://example.com/1",
		"code2": "https://example.com/2",
		"code3": "https://example.com/3",
	}

	for k, v := range entries {
		c.Set(k, v)
	}
	time.Sleep(10 * time.Millisecond)

	for k, want := range entries {
		got, found := c.Get(k)
		assert.True(t, found, "key %q should be found", k)
		assert.Equal(t, want, got, "key %q value mismatch", k)
	}
}

func TestStats_AfterOperations(t *testing.T) {
	c, err := cache.New(20)
	require.NoError(t, err)
	defer c.Close()

	// Initial stats
	hits, misses, _ := c.Stats()
	assert.Equal(t, uint64(0), hits)
	assert.Equal(t, uint64(0), misses)

	// Cause a miss
	c.Get("nonexistent")

	_, misses, _ = c.Stats()
	assert.Equal(t, uint64(1), misses)

	// Add and hit
	c.Set("key1", "value1")
	time.Sleep(10 * time.Millisecond)
	c.Get("key1")

	hits, _, ratio := c.Stats()
	assert.Equal(t, uint64(1), hits)
	assert.Equal(t, 0.5, ratio)
}
