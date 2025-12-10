package config

import (
	"time"

	"github.com/caarlos0/env/v11"
)

type Config struct {
	BaseURL     string        `env:"BASE_URL" envDefault:"http://localhost:8080"`
	SeedCount   int           `env:"SEED_COUNT" envDefault:"100000"`
	BatchSize   int           `env:"SEED_BATCH_SIZE" envDefault:"5000"`
	Rate        int           `env:"RATE" envDefault:"1000"`
	Duration    time.Duration `env:"DURATION" envDefault:"30s"`
	CreateRatio float64       `env:"CREATE_RATIO" envDefault:"0.1"`
	BenchType   string        `env:"BENCH_TYPE" envDefault:"mixed"`
}

func Load() (*Config, error) {
	var cfg Config
	if err := env.Parse(&cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}
