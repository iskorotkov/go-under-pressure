package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"golang.org/x/net/netutil"

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
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	if err := run(ctx, logger); err != nil {
		logger.Error("application failed", slog.String("error", err.Error()))
		os.Exit(1)
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

	if cfg.Pprof.Enabled {
		pprofGroup := e.Group("/debug/pprof", custommiddleware.PprofAuth(cfg.Pprof.Secret))
		custommiddleware.RegisterPprof(pprofGroup)
		logger.Info("pprof endpoints enabled", slog.String("path", "/debug/pprof/*"))
	}

	httpAddr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
	logger.Info("starting HTTP server",
		slog.String("addr", httpAddr),
		slog.Int("max_connections", cfg.Server.MaxConnections))

	httpListener, err := net.Listen("tcp", httpAddr)
	if err != nil {
		return fmt.Errorf("failed to create HTTP listener: %w", err)
	}
	if cfg.Server.MaxConnections > 0 {
		httpListener = netutil.LimitListener(httpListener, cfg.Server.MaxConnections)
	}

	httpServer := &http.Server{
		Handler:        e,
		ReadTimeout:    5 * time.Second,
		WriteTimeout:   10 * time.Second,
		IdleTimeout:    120 * time.Second,
		MaxHeaderBytes: 1 << 14, // 16KB
	}

	go func() {
		if err := httpServer.Serve(httpListener); err != nil && err != http.ErrServerClosed {
			logger.Error("http server error", slog.String("error", err.Error()))
		}
	}()

	var httpsServer *http.Server
	if cfg.TLS.Enabled {
		httpsAddr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.TLS.Port)
		logger.Info("starting HTTPS server",
			slog.String("addr", httpsAddr),
			slog.Int("max_connections", cfg.Server.MaxConnections))

		httpsListener, err := net.Listen("tcp", httpsAddr)
		if err != nil {
			return fmt.Errorf("failed to create HTTPS listener: %w", err)
		}
		if cfg.Server.MaxConnections > 0 {
			httpsListener = netutil.LimitListener(httpsListener, cfg.Server.MaxConnections)
		}

		cert, err := tls.LoadX509KeyPair(cfg.TLS.CertFile, cfg.TLS.KeyFile)
		if err != nil {
			return fmt.Errorf("failed to load TLS certificate: %w", err)
		}

		tlsListener := tls.NewListener(httpsListener, &tls.Config{
			MinVersion:             tls.VersionTLS13,
			Certificates:           []tls.Certificate{cert},
			CurvePreferences:       []tls.CurveID{tls.X25519},
			SessionTicketsDisabled: false,
		})

		httpsServer = &http.Server{
			Handler:        e,
			ReadTimeout:    5 * time.Second,
			WriteTimeout:   10 * time.Second,
			IdleTimeout:    120 * time.Second,
			MaxHeaderBytes: 1 << 14, // 16KB
		}

		go func() {
			if err := httpsServer.Serve(tlsListener); err != nil && err != http.ErrServerClosed {
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
