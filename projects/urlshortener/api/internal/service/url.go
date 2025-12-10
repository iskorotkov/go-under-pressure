package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"urlshortener/internal/cache"
	"urlshortener/internal/domain"
	"urlshortener/internal/metrics"
	"urlshortener/internal/repository"
	"urlshortener/internal/shortener"
)

var ErrURLNotFound = errors.New("url not found")

type URLService struct {
	repo      *repository.URLRepository
	shortener *shortener.Shortener
	cache     *cache.URLCache
	baseURL   string
	recorder  *metrics.Recorder
}

func NewURLService(
	repo *repository.URLRepository,
	shortener *shortener.Shortener,
	cache *cache.URLCache,
	baseURL string,
	recorder *metrics.Recorder,
) *URLService {
	return &URLService{
		repo:      repo,
		shortener: shortener,
		cache:     cache,
		baseURL:   baseURL,
		recorder:  recorder,
	}
}

func (s *URLService) CreateShortURL(ctx context.Context, originalURL string) (*domain.CreateURLResponse, error) {
	id, err := s.repo.NextID(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get next id: %w", err)
	}

	shortCode, err := s.shortener.Generate(id)
	if err != nil {
		return nil, fmt.Errorf("failed to generate short code: %w", err)
	}

	if err := s.repo.Create(ctx, id, shortCode, originalURL); err != nil {
		return nil, fmt.Errorf("failed to create url: %w", err)
	}

	s.cache.Set(shortCode, originalURL)
	s.recorder.RecordBusiness("urls_created", 1, map[string]string{"method": "single"})

	return &domain.CreateURLResponse{
		ShortCode:   shortCode,
		ShortURL:    s.baseURL + "/" + shortCode,
		OriginalURL: originalURL,
	}, nil
}

func (s *URLService) GetOriginalURL(ctx context.Context, shortCode string) (string, error) {
	if url, found := s.cache.Get(shortCode); found {
		s.recorder.RecordBusiness("cache_hit", 1, nil)
		s.recorder.RecordBusiness("redirects", 1, nil)
		return url, nil
	}

	s.recorder.RecordBusiness("cache_miss", 1, nil)

	url, err := s.repo.FindByShortCode(ctx, shortCode)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", ErrURLNotFound
		}
		return "", fmt.Errorf("failed to find url: %w", err)
	}

	s.cache.Set(shortCode, url)
	s.recorder.RecordBusiness("redirects", 1, nil)

	return url, nil
}

func (s *URLService) CreateShortURLBatch(ctx context.Context, originalURLs []string) ([]domain.CreateURLResponse, error) {
	count := len(originalURLs)
	if count == 0 {
		return []domain.CreateURLResponse{}, nil
	}

	ids, err := s.repo.NextIDs(ctx, count)
	if err != nil {
		return nil, fmt.Errorf("failed to get next ids: %w", err)
	}

	urlRows := make([]repository.URLRow, count)
	responses := make([]domain.CreateURLResponse, count)

	for i, originalURL := range originalURLs {
		shortCode, err := s.shortener.Generate(ids[i])
		if err != nil {
			return nil, fmt.Errorf("failed to generate short code: %w", err)
		}

		urlRows[i] = repository.URLRow{
			ID:          ids[i],
			ShortCode:   shortCode,
			OriginalURL: originalURL,
		}

		responses[i] = domain.CreateURLResponse{
			ShortCode:   shortCode,
			ShortURL:    s.baseURL + "/" + shortCode,
			OriginalURL: originalURL,
		}

		s.cache.Set(shortCode, originalURL)
	}

	if err := s.repo.CreateBatch(ctx, urlRows); err != nil {
		return nil, fmt.Errorf("failed to create urls: %w", err)
	}

	s.recorder.RecordBusiness("urls_created", float64(count), map[string]string{"method": "batch"})
	s.recorder.RecordBusiness("batch_size", float64(count), nil)

	return responses, nil
}
