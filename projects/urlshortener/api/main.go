package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"runtime/pprof"
	"syscall"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"

	"urlshortener/internal/cache"
	"urlshortener/internal/config"
	"urlshortener/internal/handler"
	"urlshortener/internal/metrics"
	custommiddleware "urlshortener/internal/middleware"
	"urlshortener/internal/repository"
	"urlshortener/internal/service"
	"urlshortener/internal/shortener"
	"urlshortener/internal/validation"
)

func main() {
	pprofEnabled := os.Getenv("PPROF") != ""

	if pprofEnabled {
		cpuFile, err := os.Create("cpu.prof")
		if err != nil {
			slog.Error("failed to create CPU profile", slog.String("error", err.Error()))
			os.Exit(1)
		}
		defer func() { _ = cpuFile.Close() }()

		if err := pprof.StartCPUProfile(cpuFile); err != nil {
			slog.Error("failed to start CPU profile", slog.String("error", err.Error()))
			os.Exit(1)
		}
		defer pprof.StopCPUProfile()
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	if err := run(ctx, logger); err != nil {
		logger.Error("application failed", slog.String("error", err.Error()))
		os.Exit(1)
	}

	if pprofEnabled {
		memFile, err := os.Create("mem.prof")
		if err != nil {
			logger.Error("failed to create memory profile", slog.String("error", err.Error()))
			return
		}
		defer func() { _ = memFile.Close() }()

		if err := pprof.WriteHeapProfile(memFile); err != nil {
			logger.Error("failed to write memory profile", slog.String("error", err.Error()))
		}
	}
}

func run(ctx context.Context, logger *slog.Logger) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	repo, err := repository.NewURLRepository(&cfg.Database)
	if err != nil {
		return fmt.Errorf("failed to create repository: %w", err)
	}
	defer repo.Close()

	short, err := shortener.New()
	if err != nil {
		return fmt.Errorf("failed to create shortener: %w", err)
	}

	urlCache, err := cache.New(cfg.Cache.MaxSizePow2)
	if err != nil {
		return fmt.Errorf("failed to create cache: %w", err)
	}
	defer urlCache.Close()

	recorder := metrics.NewRecorder(repo.Pool(), &cfg.Metrics, logger)
	recorder.Start(ctx)
	defer recorder.Close()

	go collectInfraMetrics(ctx, recorder, repo, urlCache)

	urlValidator := validation.NewURLValidator(
		cfg.Validation.MaxURLLength,
		cfg.Validation.MaxBatchSize,
		cfg.Validation.AllowPrivateIPs,
	)

	urlService := service.NewURLService(repo, short, urlCache, cfg.App.BaseURL, recorder)
	h := handler.New(urlService, urlValidator, logger, recorder)

	e := echo.New()
	e.HideBanner = true
	e.Use(middleware.Recover())
	e.Use(middleware.BodyLimit(cfg.Validation.MaxRequestBodySize))
	e.Use(custommiddleware.Metrics(recorder))
	e.Use(custommiddleware.RateLimit(&cfg.RateLimit, logger))

	h.Register(e)

	httpAddr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
	logger.Info("starting HTTP server", slog.String("addr", httpAddr))

	httpServer := &http.Server{
		Addr:         httpAddr,
		Handler:      e,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	go func() {
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("http server error", slog.String("error", err.Error()))
		}
	}()

	var httpsServer *http.Server
	if cfg.TLS.Enabled {
		httpsAddr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.TLS.Port)
		logger.Info("starting HTTPS server", slog.String("addr", httpsAddr))

		httpsServer = &http.Server{
			Addr:    httpsAddr,
			Handler: e,
			TLSConfig: &tls.Config{
				MinVersion: tls.VersionTLS13,
			},
			ReadTimeout:  10 * time.Second,
			WriteTimeout: 30 * time.Second,
			IdleTimeout:  120 * time.Second,
		}

		go func() {
			if err := httpsServer.ListenAndServeTLS(cfg.TLS.CertFile, cfg.TLS.KeyFile); err != nil && err != http.ErrServerClosed {
				logger.Error("https server error", slog.String("error", err.Error()))
			}
		}()
	}

	<-ctx.Done()
	logger.Info("shutting down servers")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("http server shutdown failed: %w", err)
	}

	if httpsServer != nil {
		if err := httpsServer.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("https server shutdown failed: %w", err)
		}
	}

	return nil
}

func collectInfraMetrics(ctx context.Context, recorder *metrics.Recorder, repo *repository.URLRepository, urlCache *cache.URLCache) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			poolStat := repo.Pool().Stat()
			cacheHits, cacheMisses, cacheRatio := urlCache.Stats()

			var memStats runtime.MemStats
			runtime.ReadMemStats(&memStats)

			recorder.RecordInfra(metrics.InfraMetric{
				Time:          time.Now(),
				PoolAcquired:  int(poolStat.AcquiredConns()),
				PoolIdle:      int(poolStat.IdleConns()),
				PoolTotal:     int(poolStat.TotalConns()),
				PoolMax:       int(poolStat.MaxConns()),
				CacheHits:     int64(cacheHits),
				CacheMisses:   int64(cacheMisses),
				CacheHitRatio: cacheRatio,
				Goroutines:    runtime.NumGoroutine(),
				HeapAllocMB:   float64(memStats.HeapAlloc) / 1024 / 1024,
			})
		}
	}
}
