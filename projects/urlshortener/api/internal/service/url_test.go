package service_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"urlshortener/internal/repository"
	"urlshortener/internal/service"
	"urlshortener/internal/service/mocks"
)

// CreateShortURL tests

func TestCreateShortURL_Success(t *testing.T) {
	repo := mocks.NewMockRepository(t)
	repo.EXPECT().NextID(mock.Anything).Return(uint(42), nil)
	repo.EXPECT().Create(mock.Anything, "xyz789", "https://example.com").Return(nil)

	cache := mocks.NewMockCache(t)
	cache.EXPECT().Get("xyz789").Return("", false).Maybe()
	cache.EXPECT().Set("xyz789", "https://example.com").Return()

	shortener := mocks.NewMockCodeGenerator(t)
	shortener.EXPECT().Generate(uint(42)).Return("xyz789", nil)

	recorder := mocks.NewMockBusinessRecorder(t)
	recorder.EXPECT().RecordBusiness(mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return()

	svc := service.NewURLService(repo, shortener, cache, "http://short.url", recorder)

	resp, err := svc.CreateShortURL(context.Background(), "https://example.com")
	require.NoError(t, err)

	assert.Equal(t, "xyz789", resp.ShortCode)
	assert.Equal(t, "http://short.url/xyz789", resp.ShortURL)
	assert.Equal(t, "https://example.com", resp.OriginalURL)
}

func TestCreateShortURL_NextIDError(t *testing.T) {
	expectedErr := errors.New("db connection error")

	repo := mocks.NewMockRepository(t)
	repo.EXPECT().NextID(mock.Anything).Return(uint(0), expectedErr)

	cache := mocks.NewMockCache(t)
	shortener := mocks.NewMockCodeGenerator(t)
	recorder := mocks.NewMockBusinessRecorder(t)

	svc := service.NewURLService(repo, shortener, cache, "http://short.url", recorder)

	_, err := svc.CreateShortURL(context.Background(), "https://example.com")
	require.Error(t, err)
	assert.ErrorIs(t, err, expectedErr)
}

func TestCreateShortURL_GenerateError(t *testing.T) {
	expectedErr := errors.New("shortener error")

	repo := mocks.NewMockRepository(t)
	repo.EXPECT().NextID(mock.Anything).Return(uint(1), nil)

	cache := mocks.NewMockCache(t)
	shortener := mocks.NewMockCodeGenerator(t)
	shortener.EXPECT().Generate(uint(1)).Return("", expectedErr)

	recorder := mocks.NewMockBusinessRecorder(t)

	svc := service.NewURLService(repo, shortener, cache, "http://short.url", recorder)

	_, err := svc.CreateShortURL(context.Background(), "https://example.com")
	require.Error(t, err)
	assert.ErrorIs(t, err, expectedErr)
}

func TestCreateShortURL_CreateError(t *testing.T) {
	expectedErr := errors.New("insert error")

	repo := mocks.NewMockRepository(t)
	repo.EXPECT().NextID(mock.Anything).Return(uint(1), nil)
	repo.EXPECT().Create(mock.Anything, "abc123", "https://example.com").Return(expectedErr)

	cache := mocks.NewMockCache(t)
	shortener := mocks.NewMockCodeGenerator(t)
	shortener.EXPECT().Generate(uint(1)).Return("abc123", nil)

	recorder := mocks.NewMockBusinessRecorder(t)

	svc := service.NewURLService(repo, shortener, cache, "http://short.url", recorder)

	_, err := svc.CreateShortURL(context.Background(), "https://example.com")
	require.Error(t, err)
	assert.ErrorIs(t, err, expectedErr)
}

// GetOriginalURL tests

func TestGetOriginalURL_CacheHit(t *testing.T) {
	repo := mocks.NewMockRepository(t)

	cache := mocks.NewMockCache(t)
	cache.EXPECT().Get("abc123").Return("https://cached.example.com", true)

	shortener := mocks.NewMockCodeGenerator(t)

	var recordedMetrics []string
	recorder := mocks.NewMockBusinessRecorder(t)
	recorder.EXPECT().RecordBusiness(mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Run(func(t time.Time, name string, value float64, labelsJSON []byte) {
			recordedMetrics = append(recordedMetrics, name)
		}).Return().Times(2)

	svc := service.NewURLService(repo, shortener, cache, "http://short.url", recorder)

	url, err := svc.GetOriginalURL(context.Background(), "abc123")
	require.NoError(t, err)
	assert.Equal(t, "https://cached.example.com", url)

	assert.Len(t, recordedMetrics, 2)
	assert.Equal(t, "cache_hit", recordedMetrics[0])
	assert.Equal(t, "redirects", recordedMetrics[1])
}

func TestGetOriginalURL_CacheMiss_DBFound(t *testing.T) {
	repo := mocks.NewMockRepository(t)
	repo.EXPECT().FindByShortCode(mock.Anything, "abc123").Return("https://db.example.com", nil)

	cache := mocks.NewMockCache(t)
	cache.EXPECT().Get("abc123").Return("", false)
	cache.EXPECT().Set("abc123", "https://db.example.com").Return()

	shortener := mocks.NewMockCodeGenerator(t)

	var recordedMetrics []string
	recorder := mocks.NewMockBusinessRecorder(t)
	recorder.EXPECT().RecordBusiness(mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Run(func(t time.Time, name string, value float64, labelsJSON []byte) {
			recordedMetrics = append(recordedMetrics, name)
		}).Return().Times(2)

	svc := service.NewURLService(repo, shortener, cache, "http://short.url", recorder)

	url, err := svc.GetOriginalURL(context.Background(), "abc123")
	require.NoError(t, err)
	assert.Equal(t, "https://db.example.com", url)

	assert.Len(t, recordedMetrics, 2)
	assert.Equal(t, "cache_miss", recordedMetrics[0])
}

func TestGetOriginalURL_NotFound(t *testing.T) {
	repo := mocks.NewMockRepository(t)
	repo.EXPECT().FindByShortCode(mock.Anything, "notfound").Return("", pgx.ErrNoRows)

	cache := mocks.NewMockCache(t)
	cache.EXPECT().Get("notfound").Return("", false)

	shortener := mocks.NewMockCodeGenerator(t)

	recorder := mocks.NewMockBusinessRecorder(t)
	recorder.EXPECT().RecordBusiness(mock.Anything, "cache_miss", float64(1), mock.Anything).Return()

	svc := service.NewURLService(repo, shortener, cache, "http://short.url", recorder)

	_, err := svc.GetOriginalURL(context.Background(), "notfound")
	assert.ErrorIs(t, err, service.ErrURLNotFound)
}

func TestGetOriginalURL_DBError(t *testing.T) {
	expectedErr := errors.New("db error")

	repo := mocks.NewMockRepository(t)
	repo.EXPECT().FindByShortCode(mock.Anything, "abc123").Return("", expectedErr)

	cache := mocks.NewMockCache(t)
	cache.EXPECT().Get("abc123").Return("", false)

	shortener := mocks.NewMockCodeGenerator(t)

	recorder := mocks.NewMockBusinessRecorder(t)
	recorder.EXPECT().RecordBusiness(mock.Anything, "cache_miss", float64(1), mock.Anything).Return()

	svc := service.NewURLService(repo, shortener, cache, "http://short.url", recorder)

	_, err := svc.GetOriginalURL(context.Background(), "abc123")
	require.Error(t, err)
	assert.ErrorIs(t, err, expectedErr)
}

// CreateShortURLBatch tests

func TestCreateShortURLBatch_EmptyURLs(t *testing.T) {
	repo := mocks.NewMockRepository(t)
	cache := mocks.NewMockCache(t)
	shortener := mocks.NewMockCodeGenerator(t)
	recorder := mocks.NewMockBusinessRecorder(t)

	svc := service.NewURLService(repo, shortener, cache, "http://short.url", recorder)

	resp, err := svc.CreateShortURLBatch(context.Background(), []string{})
	require.NoError(t, err)
	assert.Empty(t, resp)
}

func TestCreateShortURLBatch_Success(t *testing.T) {
	repo := mocks.NewMockRepository(t)
	repo.EXPECT().NextIDs(mock.Anything, 2).Return([]uint{1, 2}, nil)
	repo.EXPECT().CreateBatch(mock.Anything, mock.Anything).Return(nil)

	cache := mocks.NewMockCache(t)
	cache.EXPECT().Set(mock.Anything, mock.Anything).Return().Times(2)

	shortener := mocks.NewMockCodeGenerator(t)
	shortener.EXPECT().Generate(uint(1)).Return("code1", nil)
	shortener.EXPECT().Generate(uint(2)).Return("code2", nil)

	recorder := mocks.NewMockBusinessRecorder(t)
	recorder.EXPECT().RecordBusiness(mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return().Times(2)

	svc := service.NewURLService(repo, shortener, cache, "http://short.url", recorder)

	urls := []string{"https://example.com/1", "https://example.com/2"}
	resp, err := svc.CreateShortURLBatch(context.Background(), urls)
	require.NoError(t, err)
	assert.Len(t, resp, 2)
}

func TestCreateShortURLBatch_NextIDsError(t *testing.T) {
	expectedErr := errors.New("sequence error")

	repo := mocks.NewMockRepository(t)
	repo.EXPECT().NextIDs(mock.Anything, 1).Return(nil, expectedErr)

	cache := mocks.NewMockCache(t)
	shortener := mocks.NewMockCodeGenerator(t)
	recorder := mocks.NewMockBusinessRecorder(t)

	svc := service.NewURLService(repo, shortener, cache, "http://short.url", recorder)

	_, err := svc.CreateShortURLBatch(context.Background(), []string{"https://example.com"})
	require.Error(t, err)
	assert.ErrorIs(t, err, expectedErr)
}

func TestCreateShortURLBatch_GenerateError(t *testing.T) {
	expectedErr := errors.New("generate error")

	repo := mocks.NewMockRepository(t)
	repo.EXPECT().NextIDs(mock.Anything, 2).Return([]uint{1, 2}, nil)

	cache := mocks.NewMockCache(t)
	cache.EXPECT().Set("code1", "url1").Return()

	shortener := mocks.NewMockCodeGenerator(t)
	shortener.EXPECT().Generate(uint(1)).Return("code1", nil)
	shortener.EXPECT().Generate(uint(2)).Return("", expectedErr)

	recorder := mocks.NewMockBusinessRecorder(t)

	svc := service.NewURLService(repo, shortener, cache, "http://short.url", recorder)

	_, err := svc.CreateShortURLBatch(context.Background(), []string{"url1", "url2"})
	require.Error(t, err)
	assert.ErrorIs(t, err, expectedErr)
}

func TestCreateShortURLBatch_CreateBatchError(t *testing.T) {
	expectedErr := errors.New("batch insert error")

	repo := mocks.NewMockRepository(t)
	repo.EXPECT().NextIDs(mock.Anything, 1).Return([]uint{1}, nil)
	repo.EXPECT().CreateBatch(mock.Anything, mock.MatchedBy(func(urls []repository.URLRow) bool {
		return len(urls) == 1
	})).Return(expectedErr)

	cache := mocks.NewMockCache(t)
	cache.EXPECT().Set("abc123", "https://example.com").Return()

	shortener := mocks.NewMockCodeGenerator(t)
	shortener.EXPECT().Generate(uint(1)).Return("abc123", nil)

	recorder := mocks.NewMockBusinessRecorder(t)

	svc := service.NewURLService(repo, shortener, cache, "http://short.url", recorder)

	_, err := svc.CreateShortURLBatch(context.Background(), []string{"https://example.com"})
	require.Error(t, err)
	assert.ErrorIs(t, err, expectedErr)
}
