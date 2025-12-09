package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"runtime/pprof"
	"syscall"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"

	"urlshortener/internal/cache"
	"urlshortener/internal/config"
	"urlshortener/internal/handler"
	"urlshortener/internal/repository"
	"urlshortener/internal/service"
	"urlshortener/internal/shortener"
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

	urlCache, err := cache.New()
	if err != nil {
		return fmt.Errorf("failed to create cache: %w", err)
	}
	defer urlCache.Close()

	urlService := service.NewURLService(repo, short, urlCache, cfg.App.BaseURL)
	h := handler.New(urlService, logger)

	e := echo.New()
	e.HideBanner = true
	e.Use(middleware.Recover())

	h.Register(e)

	addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
	logger.Info("starting server", slog.String("addr", addr))

	go func() {
		if err := e.Start(addr); err != nil && err != http.ErrServerClosed {
			logger.Error("server error", slog.String("error", err.Error()))
		}
	}()

	<-ctx.Done()
	logger.Info("shutting down server")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	return e.Shutdown(shutdownCtx)
}
