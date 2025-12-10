package middleware

import (
	"crypto/subtle"
	"log/slog"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"golang.org/x/time/rate"

	"urlshortener/internal/config"
)

type rateLimitResponse struct {
	Error      string `json:"error"`
	RetryAfter int    `json:"retry_after"`
}

const retryAfterHeader = "1"

var (
	rateLimitExceededResp = rateLimitResponse{
		Error:      "rate limit exceeded",
		RetryAfter: 1,
	}
	rateLimiterInternalErr = map[string]string{
		"error": "internal server error",
	}
)

const bypassHeader = "X-Rate-Limit-Bypass"

func RateLimit(cfg *config.RateLimitConfig, logger *slog.Logger) echo.MiddlewareFunc {
	store := middleware.NewRateLimiterMemoryStoreWithConfig(
		middleware.RateLimiterMemoryStoreConfig{
			Rate:      rate.Limit(cfg.RPS),
			Burst:     cfg.Burst,
			ExpiresIn: time.Duration(cfg.ExpireMinutes) * time.Minute,
		},
	)

	secret := []byte(cfg.BypassSecret)
	return middleware.RateLimiterWithConfig(middleware.RateLimiterConfig{
		Store: store,
		Skipper: func(c echo.Context) bool {
			if cfg.BypassSecret == "" {
				return false
			}
			provided := c.Request().Header.Get(bypassHeader)
			return subtle.ConstantTimeCompare([]byte(provided), secret) == 1
		},
		IdentifierExtractor: func(c echo.Context) (string, error) {
			return c.RealIP(), nil
		},
		DenyHandler: func(c echo.Context, identifier string, err error) error {
			logger.Warn("rate limit exceeded",
				slog.String("ip", identifier),
				slog.String("path", c.Path()),
			)
			c.Response().Header().Set("Retry-After", retryAfterHeader)
			return c.JSON(http.StatusTooManyRequests, rateLimitExceededResp)
		},
		ErrorHandler: func(c echo.Context, err error) error {
			logger.Error("rate limiter error", slog.String("error", err.Error()))
			return c.JSON(http.StatusInternalServerError, rateLimiterInternalErr)
		},
	})
}
