package repository

import (
	"context"
	"fmt"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"urlshortener/internal/config"
	"urlshortener/internal/domain"
)

type URLRepository struct {
	db *gorm.DB
}

func NewURLRepository(cfg *config.DatabaseConfig) (*URLRepository, error) {
	dsn := fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		cfg.Host, cfg.Port, cfg.User, cfg.Password, cfg.DBName, cfg.SSLMode,
	)

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	if err := db.AutoMigrate(&domain.URL{}); err != nil {
		return nil, fmt.Errorf("failed to run migrations: %w", err)
	}

	return &URLRepository{db: db}, nil
}

func (r *URLRepository) NextID(ctx context.Context) (uint, error) {
	var nextID uint
	err := r.db.WithContext(ctx).Raw("SELECT nextval('urls_id_seq')").Scan(&nextID).Error
	if err != nil {
		return 0, fmt.Errorf("failed to get next id: %w", err)
	}
	return nextID, nil
}

func (r *URLRepository) Create(ctx context.Context, url *domain.URL) error {
	return r.db.WithContext(ctx).Create(url).Error
}

func (r *URLRepository) FindByShortCode(ctx context.Context, shortCode string) (*domain.URL, error) {
	var url domain.URL
	err := r.db.WithContext(ctx).Where("short_code = ?", shortCode).First(&url).Error
	if err != nil {
		return nil, err
	}
	return &url, nil
}
