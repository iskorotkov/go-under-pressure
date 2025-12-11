package main

import (
	"fmt"
	"os"

	"bench/internal/attack"
	"bench/internal/config"
	"bench/internal/seed"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	var codes []string
	if cfg.BenchType != "create" {
		var err error
		codes, err = seed.Run(cfg.BaseURL, cfg.SeedCount, cfg.BatchSize, cfg.RateLimitBypass, cfg.InsecureSkipVerify, cfg.SeedTimeout)
		if err != nil {
			return fmt.Errorf("seed failed: %w", err)
		}
	}

	return attack.Run(&attack.Config{
		BaseURL:            cfg.BaseURL,
		Codes:              codes,
		Rate:               cfg.Rate,
		Duration:           cfg.Duration,
		CreateRatio:        cfg.CreateRatio,
		Type:               cfg.BenchType,
		RateLimitBypass:    cfg.RateLimitBypass,
		InsecureSkipVerify: cfg.InsecureSkipVerify,
		Connections:        cfg.Connections,
		MaxWorkers:         cfg.MaxWorkers,
	})
}
