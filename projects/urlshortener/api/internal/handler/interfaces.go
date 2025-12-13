package handler

import (
	"context"
	"time"

	"urlshortener/internal/domain"
)

type URLService interface {
	CreateShortURL(ctx context.Context, originalURL string) (*domain.CreateURLResponse, error)
	GetOriginalURL(ctx context.Context, shortCode string) (string, error)
	CreateShortURLBatch(ctx context.Context, originalURLs []string) ([]domain.CreateURLResponse, error)
}

type URLValidator interface {
	ValidateURL(url string) error
	ValidateBatch(urls []string) error
}

type BusinessRecorder interface {
	RecordBusiness(t time.Time, name string, value float64, labelsJSON []byte)
}
