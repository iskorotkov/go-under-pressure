package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"urlshortener/internal/cache"
	"urlshortener/internal/domain"
	"urlshortener/internal/repository"
	"urlshortener/internal/shortener"
)

var ErrURLNotFound = errors.New("url not found")

type URLService struct {
	repo      *repository.URLRepository
	shortener *shortener.Shortener
	cache     *cache.URLCache
	baseURL   string
}

func NewURLService(repo *repository.URLRepository, shortener *shortener.Shortener, cache *cache.URLCache, baseURL string) *URLService {
	return &URLService{
		repo:      repo,
		shortener: shortener,
		cache:     cache,
		baseURL:   baseURL,
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

	// Cache the new URL
	s.cache.Set(shortCode, originalURL)

	return &domain.CreateURLResponse{
		ShortCode:   shortCode,
		ShortURL:    fmt.Sprintf("%s/%s", s.baseURL, shortCode),
		OriginalURL: originalURL,
	}, nil
}

func (s *URLService) GetOriginalURL(ctx context.Context, shortCode string) (string, error) {
	// Check cache first
	if url, found := s.cache.Get(shortCode); found {
		return url, nil
	}

	url, err := s.repo.FindByShortCode(ctx, shortCode)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", ErrURLNotFound
		}
		return "", fmt.Errorf("failed to find url: %w", err)
	}

	// Cache the result
	s.cache.Set(shortCode, url)

	return url, nil
}
