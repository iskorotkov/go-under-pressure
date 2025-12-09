package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"urlshortener/internal/config"
	"urlshortener/internal/domain"
)

type URLRepository struct {
	pool *pgxpool.Pool
}

func NewURLRepository(cfg *config.DatabaseConfig) (*URLRepository, error) {
	dsn := fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		cfg.Host, cfg.Port, cfg.User, cfg.Password, cfg.DBName, cfg.SSLMode,
	)

	// Use GORM only for migrations
	gormDB, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("failed to connect for migrations: %w", err)
	}

	if err := gormDB.AutoMigrate(&domain.URL{}); err != nil {
		return nil, fmt.Errorf("failed to run migrations: %w", err)
	}

	// Close GORM connection after migrations
	sqlDB, err := gormDB.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get sql.DB: %w", err)
	}
	_ = sqlDB.Close()

	// Create pgx pool for queries
	poolConfig, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to parse pool config: %w", err)
	}

	poolConfig.MaxConns = 50
	poolConfig.MinConns = 10
	poolConfig.MaxConnLifetime = 5 * time.Minute
	poolConfig.MaxConnIdleTime = 1 * time.Minute

	pool, err := pgxpool.NewWithConfig(context.Background(), poolConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create connection pool: %w", err)
	}

	return &URLRepository{pool: pool}, nil
}

func (r *URLRepository) Close() {
	r.pool.Close()
}

func (r *URLRepository) NextID(ctx context.Context) (uint, error) {
	var id uint
	err := r.pool.QueryRow(ctx, "SELECT nextval('urls_id_seq')").Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("failed to get next id: %w", err)
	}
	return id, nil
}

func (r *URLRepository) Create(ctx context.Context, id uint, shortCode, originalURL string) error {
	_, err := r.pool.Exec(ctx,
		"INSERT INTO urls (id, short_code, original_url, created_at, updated_at) VALUES ($1, $2, $3, NOW(), NOW())",
		id, shortCode, originalURL,
	)
	if err != nil {
		return fmt.Errorf("failed to create url: %w", err)
	}
	return nil
}

func (r *URLRepository) FindByShortCode(ctx context.Context, shortCode string) (string, error) {
	var originalURL string
	err := r.pool.QueryRow(ctx,
		"SELECT original_url FROM urls WHERE short_code = $1",
		shortCode,
	).Scan(&originalURL)
	if err != nil {
		return "", err
	}
	return originalURL, nil
}
