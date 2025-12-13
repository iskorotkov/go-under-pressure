package middleware

import (
	"cmp"
	"time"

	"github.com/labstack/echo/v4"

	"urlshortener/internal/metrics"
)

type HTTPRecorder interface {
	RecordHTTP(m metrics.HTTPMetric)
}

func Metrics(recorder HTTPRecorder) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			start := time.Now()

			err := next(c)

			duration := time.Since(start)
			path := cmp.Or(c.Path(), "/")
			statusCode := c.Response().Status

			var errStr string
			if err != nil {
				errStr = err.Error()
				if he, ok := err.(*echo.HTTPError); ok {
					statusCode = he.Code
				}
			}

			recorder.RecordHTTP(metrics.HTTPMetric{
				Time:       start,
				Method:     c.Request().Method,
				Path:       path,
				StatusCode: statusCode,
				DurationMs: float64(duration.Microseconds()) / 1000.0,
				ClientIP:   c.RealIP(),
				Error:      errStr,
			})

			return err
		}
	}
}
