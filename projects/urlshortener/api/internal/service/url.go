package service

import (
	"context"
	"errors"
	"fmt"

	"gorm.io/gorm"

	"urlshortener/internal/domain"
	"urlshortener/internal/repository"
	"urlshortener/internal/shortener"
)

var ErrURLNotFound = errors.New("url not found")

type URLService struct {
	repo      *repository.URLRepository
	shortener *shortener.Shortener
	baseURL   string
}

func NewURLService(repo *repository.URLRepository, shortener *shortener.Shortener, baseURL string) *URLService {
	return &URLService{
		repo:      repo,
		shortener: shortener,
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

	url := &domain.URL{
		ID:          id,
		ShortCode:   shortCode,
		OriginalURL: originalURL,
	}

	if err := s.repo.Create(ctx, url); err != nil {
		return nil, fmt.Errorf("failed to create url: %w", err)
	}

	return &domain.CreateURLResponse{
		ShortCode:   shortCode,
		ShortURL:    fmt.Sprintf("%s/%s", s.baseURL, shortCode),
		OriginalURL: originalURL,
	}, nil
}

func (s *URLService) GetOriginalURL(ctx context.Context, shortCode string) (string, error) {
	url, err := s.repo.FindByShortCode(ctx, shortCode)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", ErrURLNotFound
		}
		return "", fmt.Errorf("failed to find url: %w", err)
	}
	return url.OriginalURL, nil
}
