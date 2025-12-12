package service

//go:generate go tool mockery

import (
	"context"
	"time"

	"urlshortener/internal/repository"
)

type Repository interface {
	NextID(ctx context.Context) (uint, error)
	Create(ctx context.Context, id uint, shortCode, originalURL string) error
	FindByShortCode(ctx context.Context, shortCode string) (string, error)
	NextIDs(ctx context.Context, count int) ([]uint, error)
	CreateBatch(ctx context.Context, urls []repository.URLRow) error
}

type Cache interface {
	Get(shortCode string) (string, bool)
	Set(shortCode, originalURL string)
}

type CodeGenerator interface {
	Generate(id uint) (string, error)
}

type BusinessRecorder interface {
	RecordBusiness(t time.Time, name string, value float64, labelsJSON []byte)
}
