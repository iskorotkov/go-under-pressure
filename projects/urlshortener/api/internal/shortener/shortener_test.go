package shortener_test

import (
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"urlshortener/internal/shortener"
)

func TestNew(t *testing.T) {
	s, err := shortener.New()
	require.NoError(t, err)
	assert.NotNil(t, s)
}

func TestGenerate_MinLength(t *testing.T) {
	s, err := shortener.New()
	require.NoError(t, err)

	code, err := s.Generate(0)
	require.NoError(t, err)
	assert.Len(t, code, 6) // ID 0 produces exactly minimum length
}

func TestGenerate_DifferentIDs(t *testing.T) {
	s, err := shortener.New()
	require.NoError(t, err)

	code0, err := s.Generate(0)
	require.NoError(t, err)
	assert.Equal(t, "bMZn4Y", code0)

	code1, err := s.Generate(1)
	require.NoError(t, err)
	assert.Equal(t, "UkLWZg", code1)
}

func TestGenerate_LargeID(t *testing.T) {
	s, err := shortener.New()
	require.NoError(t, err)

	largeID := uint(1_000_000_000)
	code, err := s.Generate(largeID)
	require.NoError(t, err)
	assert.Len(t, code, 7) // Large ID produces 7-char code
}

func TestGenerate_URLSafe(t *testing.T) {
	s, err := shortener.New()
	require.NoError(t, err)

	urlSafePattern := regexp.MustCompile(`^[a-zA-Z0-9]+$`)

	ids := []uint{0, 1, 100, 1000, 10000, 100000, 1000000}
	for _, id := range ids {
		code, err := s.Generate(id)
		require.NoError(t, err)
		assert.Regexp(t, urlSafePattern, code)
	}
}

func TestGenerate_Deterministic(t *testing.T) {
	s, err := shortener.New()
	require.NoError(t, err)

	code, err := s.Generate(12345)
	require.NoError(t, err)
	assert.Equal(t, "A6das1", code)
}
