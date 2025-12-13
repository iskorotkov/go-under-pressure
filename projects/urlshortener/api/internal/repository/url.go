package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"urlshortener/internal/config"
)

type URLRepository struct {
	pool *pgxpool.Pool
}

func NewURLRepository(cfg *config.DatabaseConfig) (*URLRepository, error) {
	dsn := fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		cfg.Host, cfg.Port, cfg.User, cfg.Password, cfg.DBName, cfg.SSLMode,
	)

	poolConfig, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to parse pool config: %w", err)
	}

	poolConfig.MaxConns = int32(cfg.PoolMaxConns)
	poolConfig.MinConns = int32(cfg.PoolMinConns)
	poolConfig.MaxConnLifetime = time.Duration(cfg.PoolMaxConnLifetime) * time.Minute
	poolConfig.MaxConnIdleTime = time.Duration(cfg.PoolMaxConnIdleTime) * time.Minute
	poolConfig.MaxConnLifetimeJitter = 2 * time.Minute

	pool, err := pgxpool.NewWithConfig(context.Background(), poolConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create connection pool: %w", err)
	}

	return &URLRepository{pool: pool}, nil
}

func (r *URLRepository) Close() {
	r.pool.Close()
}

func (r *URLRepository) Pool() *pgxpool.Pool {
	return r.pool
}

func (r *URLRepository) NextID(ctx context.Context) (uint, error) {
	var id uint
	err := r.pool.QueryRow(ctx, "SELECT nextval('urls_id_seq')").Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("failed to get next id: %w", err)
	}
	return id, nil
}

func (r *URLRepository) Create(ctx context.Context, shortCode, originalURL string) error {
	_, err := r.pool.Exec(ctx,
		"INSERT INTO urls (short_code, original_url, created_at) VALUES ($1, $2, NOW())",
		shortCode, originalURL,
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

func (r *URLRepository) NextIDs(ctx context.Context, count int) ([]uint, error) {
	rows, err := r.pool.Query(ctx,
		"SELECT nextval('urls_id_seq') FROM generate_series(1, $1)",
		count,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get next ids: %w", err)
	}
	defer rows.Close()

	ids := make([]uint, 0, count)
	for rows.Next() {
		var id uint
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("failed to scan id: %w", err)
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

type URLRow struct {
	ShortCode   string
	OriginalURL string
}

func (r *URLRepository) CreateBatch(ctx context.Context, urls []URLRow) error {
	now := time.Now()
	rows := make([][]any, len(urls))
	for i, u := range urls {
		rows[i] = []any{u.ShortCode, u.OriginalURL, now}
	}

	_, err := r.pool.CopyFrom(
		ctx,
		pgx.Identifier{"urls"},
		[]string{"short_code", "original_url", "created_at"},
		pgx.CopyFromRows(rows),
	)
	if err != nil {
		return fmt.Errorf("failed to batch insert urls: %w", err)
	}
	return nil
}
