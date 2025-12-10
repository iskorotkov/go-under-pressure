package middleware_test

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

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

		assert.Equal(t, http.StatusOK, rec.Code, "request %d should succeed", i)
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

	assert.True(t, rateLimited, "expected at least one request to be rate limited")
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

	// First request uses up the burst
	req1 := httptest.NewRequest(http.MethodGet, "/test", nil)
	req1.RemoteAddr = "192.168.1.3:12345"
	rec1 := httptest.NewRecorder()
	e.ServeHTTP(rec1, req1)

	// Second request should be rate limited
	req2 := httptest.NewRequest(http.MethodGet, "/test", nil)
	req2.RemoteAddr = "192.168.1.3:12345"
	rec2 := httptest.NewRecorder()
	e.ServeHTTP(rec2, req2)

	require.Equal(t, http.StatusTooManyRequests, rec2.Code)
	assert.Equal(t, "1", rec2.Header().Get("Retry-After"))

	var resp struct {
		Error      string `json:"error"`
		RetryAfter int    `json:"retry_after"`
	}
	err := json.Unmarshal(rec2.Body.Bytes(), &resp)
	require.NoError(t, err)

	assert.Equal(t, "rate limit exceeded", resp.Error)
	assert.Equal(t, 1, resp.RetryAfter)
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
	assert.Equal(t, http.StatusOK, rec1.Code, "IP1 first request should succeed")

	req2 := httptest.NewRequest(http.MethodGet, "/test", nil)
	req2.RemoteAddr = "192.168.1.5:12345"
	rec2 := httptest.NewRecorder()
	e.ServeHTTP(rec2, req2)
	assert.Equal(t, http.StatusOK, rec2.Code, "IP2 first request should succeed")
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

		assert.Equal(t, http.StatusOK, rec.Code, "request %d with bypass should succeed", i)
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

	// First request uses up the burst
	req1 := httptest.NewRequest(http.MethodGet, "/test", nil)
	req1.RemoteAddr = "192.168.1.7:12345"
	req1.Header.Set("X-Rate-Limit-Bypass", "wrong_secret")
	rec1 := httptest.NewRecorder()
	e.ServeHTTP(rec1, req1)

	// Second request should be rate limited
	req2 := httptest.NewRequest(http.MethodGet, "/test", nil)
	req2.RemoteAddr = "192.168.1.7:12345"
	req2.Header.Set("X-Rate-Limit-Bypass", "wrong_secret")
	rec2 := httptest.NewRecorder()
	e.ServeHTTP(rec2, req2)

	assert.Equal(t, http.StatusTooManyRequests, rec2.Code, "wrong secret should not bypass rate limit")
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

	// First request uses up the burst
	req1 := httptest.NewRequest(http.MethodGet, "/test", nil)
	req1.RemoteAddr = "192.168.1.8:12345"
	req1.Header.Set("X-Rate-Limit-Bypass", "any_value")
	rec1 := httptest.NewRecorder()
	e.ServeHTTP(rec1, req1)

	// Second request should be rate limited
	req2 := httptest.NewRequest(http.MethodGet, "/test", nil)
	req2.RemoteAddr = "192.168.1.8:12345"
	req2.Header.Set("X-Rate-Limit-Bypass", "any_value")
	rec2 := httptest.NewRecorder()
	e.ServeHTTP(rec2, req2)

	assert.Equal(t, http.StatusTooManyRequests, rec2.Code, "bypass should be disabled when secret is empty")
}
