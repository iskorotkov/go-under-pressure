package config

import "github.com/caarlos0/env/v11"

type Config struct {
	Server   ServerConfig
	Database DatabaseConfig
	App      AppConfig
	Cache    CacheConfig
}

type ServerConfig struct {
	Host string `env:"SERVER_HOST" envDefault:"localhost"`
	Port int    `env:"SERVER_PORT" envDefault:"8080"`
}

type DatabaseConfig struct {
	Host     string `env:"POSTGRES_HOST" envDefault:"localhost"`
	Port     int    `env:"POSTGRES_PORT" envDefault:"5432"`
	User     string `env:"POSTGRES_USER" envDefault:"postgres"`
	Password string `env:"POSTGRES_PASSWORD" envDefault:"postgres"`
	DBName   string `env:"POSTGRES_DB" envDefault:"urlshortener"`
	SSLMode  string `env:"POSTGRES_SSLMODE" envDefault:"disable"`
}

type AppConfig struct {
	BaseURL string `env:"BASE_URL" envDefault:"http://localhost:8080"`
}

type CacheConfig struct {
	MaxSizePow2 int `env:"CACHE_MAX_SIZE_POW2" envDefault:"0"` // 2^27 = 128MB
}

func Load() (*Config, error) {
	var cfg Config
	if err := env.Parse(&cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}
