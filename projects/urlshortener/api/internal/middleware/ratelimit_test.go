package middleware_test

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/labstack/echo/v4"

	"urlshortener/internal/config"
	"urlshortener/internal/middleware"
)

func TestRateLimit_AllowsRequestsUnderLimit(t *testing.T) {
	e := echo.New()
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	cfg := &config.RateLimitConfig{
		RPS:           10,
		Burst:         5,
		ExpireMinutes: 1,
	}

	e.Use(middleware.RateLimit(cfg, logger))
	e.GET("/test", func(c echo.Context) error {
		return c.String(http.StatusOK, "ok")
	})

	for i := 0; i < 5; i++ {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.RemoteAddr = "192.168.1.1:12345"
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("request %d: expected status %d, got %d", i, http.StatusOK, rec.Code)
		}
	}
}

func TestRateLimit_BlocksRequestsOverLimit(t *testing.T) {
	e := echo.New()
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	cfg := &config.RateLimitConfig{
		RPS:           1,
		Burst:         2,
		ExpireMinutes: 1,
	}

	e.Use(middleware.RateLimit(cfg, logger))
	e.GET("/test", func(c echo.Context) error {
		return c.String(http.StatusOK, "ok")
	})

	var rateLimited bool
	for i := 0; i < 10; i++ {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.RemoteAddr = "192.168.1.2:12345"
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)

		if rec.Code == http.StatusTooManyRequests {
			rateLimited = true
			break
		}
	}

	if !rateLimited {
		t.Error("expected at least one request to be rate limited")
	}
}

func TestRateLimit_Returns429WithCorrectResponse(t *testing.T) {
	e := echo.New()
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	cfg := &config.RateLimitConfig{
		RPS:           0.1,
		Burst:         1,
		ExpireMinutes: 1,
	}

	e.Use(middleware.RateLimit(cfg, logger))
	e.GET("/test", func(c echo.Context) error {
		return c.String(http.StatusOK, "ok")
	})

	req1 := httptest.NewRequest(http.MethodGet, "/test", nil)
	req1.RemoteAddr = "192.168.1.3:12345"
	rec1 := httptest.NewRecorder()
	e.ServeHTTP(rec1, req1)

	req2 := httptest.NewRequest(http.MethodGet, "/test", nil)
	req2.RemoteAddr = "192.168.1.3:12345"
	rec2 := httptest.NewRecorder()
	e.ServeHTTP(rec2, req2)

	if rec2.Code != http.StatusTooManyRequests {
		t.Fatalf("expected status %d, got %d", http.StatusTooManyRequests, rec2.Code)
	}

	retryAfter := rec2.Header().Get("Retry-After")
	if retryAfter != "1" {
		t.Errorf("expected Retry-After header to be '1', got '%s'", retryAfter)
	}

	var resp struct {
		Error      string `json:"error"`
		RetryAfter int    `json:"retry_after"`
	}
	if err := json.Unmarshal(rec2.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if resp.Error != "rate limit exceeded" {
		t.Errorf("expected error 'rate limit exceeded', got '%s'", resp.Error)
	}
	if resp.RetryAfter != 1 {
		t.Errorf("expected retry_after 1, got %d", resp.RetryAfter)
	}
}

func TestRateLimit_DifferentIPsHaveSeparateLimits(t *testing.T) {
	e := echo.New()
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	cfg := &config.RateLimitConfig{
		RPS:           0.1,
		Burst:         1,
		ExpireMinutes: 1,
	}

	e.Use(middleware.RateLimit(cfg, logger))
	e.GET("/test", func(c echo.Context) error {
		return c.String(http.StatusOK, "ok")
	})

	req1 := httptest.NewRequest(http.MethodGet, "/test", nil)
	req1.RemoteAddr = "192.168.1.4:12345"
	rec1 := httptest.NewRecorder()
	e.ServeHTTP(rec1, req1)

	if rec1.Code != http.StatusOK {
		t.Errorf("IP1 first request: expected status %d, got %d", http.StatusOK, rec1.Code)
	}

	req2 := httptest.NewRequest(http.MethodGet, "/test", nil)
	req2.RemoteAddr = "192.168.1.5:12345"
	rec2 := httptest.NewRecorder()
	e.ServeHTTP(rec2, req2)

	if rec2.Code != http.StatusOK {
		t.Errorf("IP2 first request: expected status %d, got %d", http.StatusOK, rec2.Code)
	}
}

func TestRateLimit_BypassWithCorrectSecret(t *testing.T) {
	e := echo.New()
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	cfg := &config.RateLimitConfig{
		RPS:           0.1,
		Burst:         1,
		ExpireMinutes: 1,
		BypassSecret:  "test_secret",
	}

	e.Use(middleware.RateLimit(cfg, logger))
	e.GET("/test", func(c echo.Context) error {
		return c.String(http.StatusOK, "ok")
	})

	for i := 0; i < 10; i++ {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.RemoteAddr = "192.168.1.6:12345"
		req.Header.Set("X-Rate-Limit-Bypass", "test_secret")
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("request %d: expected status %d, got %d", i, http.StatusOK, rec.Code)
		}
	}
}

func TestRateLimit_BypassWithWrongSecret(t *testing.T) {
	e := echo.New()
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	cfg := &config.RateLimitConfig{
		RPS:           0.1,
		Burst:         1,
		ExpireMinutes: 1,
		BypassSecret:  "test_secret",
	}

	e.Use(middleware.RateLimit(cfg, logger))
	e.GET("/test", func(c echo.Context) error {
		return c.String(http.StatusOK, "ok")
	})

	req1 := httptest.NewRequest(http.MethodGet, "/test", nil)
	req1.RemoteAddr = "192.168.1.7:12345"
	req1.Header.Set("X-Rate-Limit-Bypass", "wrong_secret")
	rec1 := httptest.NewRecorder()
	e.ServeHTTP(rec1, req1)

	req2 := httptest.NewRequest(http.MethodGet, "/test", nil)
	req2.RemoteAddr = "192.168.1.7:12345"
	req2.Header.Set("X-Rate-Limit-Bypass", "wrong_secret")
	rec2 := httptest.NewRecorder()
	e.ServeHTTP(rec2, req2)

	if rec2.Code != http.StatusTooManyRequests {
		t.Errorf("expected status %d with wrong secret, got %d", http.StatusTooManyRequests, rec2.Code)
	}
}

func TestRateLimit_BypassDisabledWhenSecretEmpty(t *testing.T) {
	e := echo.New()
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	cfg := &config.RateLimitConfig{
		RPS:           0.1,
		Burst:         1,
		ExpireMinutes: 1,
		BypassSecret:  "",
	}

	e.Use(middleware.RateLimit(cfg, logger))
	e.GET("/test", func(c echo.Context) error {
		return c.String(http.StatusOK, "ok")
	})

	req1 := httptest.NewRequest(http.MethodGet, "/test", nil)
	req1.RemoteAddr = "192.168.1.8:12345"
	req1.Header.Set("X-Rate-Limit-Bypass", "any_value")
	rec1 := httptest.NewRecorder()
	e.ServeHTTP(rec1, req1)

	req2 := httptest.NewRequest(http.MethodGet, "/test", nil)
	req2.RemoteAddr = "192.168.1.8:12345"
	req2.Header.Set("X-Rate-Limit-Bypass", "any_value")
	rec2 := httptest.NewRecorder()
	e.ServeHTTP(rec2, req2)

	if rec2.Code != http.StatusTooManyRequests {
		t.Errorf("expected status %d when bypass disabled, got %d", http.StatusTooManyRequests, rec2.Code)
	}
}
