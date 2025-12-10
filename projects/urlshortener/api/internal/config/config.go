package config

import "github.com/caarlos0/env/v11"

type Config struct {
	Server     ServerConfig
	Database   DatabaseConfig
	App        AppConfig
	Cache      CacheConfig
	RateLimit  RateLimitConfig
	Metrics    MetricsConfig
	Validation ValidationConfig
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

type RateLimitConfig struct {
	RPS           float64 `env:"RATE_LIMIT_RPS" envDefault:"100"`
	Burst         int     `env:"RATE_LIMIT_BURST" envDefault:"200"`
	ExpireMinutes int     `env:"RATE_LIMIT_EXPIRE_MINUTES" envDefault:"3"`
	BypassSecret  string  `env:"RATE_LIMIT_BYPASS_SECRET"`
}

type MetricsConfig struct {
	Enabled        bool `env:"METRICS_ENABLED" envDefault:"true"`
	BufferSize     int  `env:"METRICS_BUFFER_SIZE" envDefault:"10000"`
	FlushInterval  int  `env:"METRICS_FLUSH_INTERVAL_MS" envDefault:"100"`
	FlushThreshold int  `env:"METRICS_FLUSH_THRESHOLD" envDefault:"1000"`
}

type ValidationConfig struct {
	MaxURLLength       int    `env:"VALIDATION_MAX_URL_LENGTH" envDefault:"2048"`
	MaxBatchSize       int    `env:"VALIDATION_MAX_BATCH_SIZE" envDefault:"5000"`
	MaxRequestBodySize string `env:"VALIDATION_MAX_BODY_SIZE" envDefault:"1M"`
	AllowPrivateIPs    bool   `env:"VALIDATION_ALLOW_PRIVATE_IPS" envDefault:"false"`
}

func Load() (*Config, error) {
	var cfg Config
	if err := env.Parse(&cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}
