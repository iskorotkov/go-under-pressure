package middleware_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"urlshortener/internal/metrics"
	"urlshortener/internal/middleware"
	"urlshortener/internal/middleware/mocks"
)

func TestMetrics_SuccessfulRequest(t *testing.T) {
	rec := mocks.NewMockHTTPRecorder(t)

	var capturedMetric metrics.HTTPMetric
	rec.EXPECT().RecordHTTP(mock.Anything).
		Run(func(m metrics.HTTPMetric) {
			capturedMetric = m
		}).Return().Once()

	mw := middleware.Metrics(rec)

	e := echo.New()
	e.Use(mw)
	e.GET("/test", func(c echo.Context) error {
		return c.String(http.StatusOK, "ok")
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.RemoteAddr = "192.168.1.1:12345"
	resp := httptest.NewRecorder()
	e.ServeHTTP(resp, req)

	assert.Equal(t, http.MethodGet, capturedMetric.Method)
	assert.Equal(t, "/test", capturedMetric.Path)
	assert.Equal(t, http.StatusOK, capturedMetric.StatusCode)
	assert.GreaterOrEqual(t, capturedMetric.DurationMs, 0.0)
	assert.Equal(t, "192.168.1.1", capturedMetric.ClientIP)
	assert.Empty(t, capturedMetric.Error)
}

func TestMetrics_RequestWithError(t *testing.T) {
	rec := mocks.NewMockHTTPRecorder(t)

	var capturedMetric metrics.HTTPMetric
	rec.EXPECT().RecordHTTP(mock.Anything).
		Run(func(m metrics.HTTPMetric) {
			capturedMetric = m
		}).Return().Once()

	mw := middleware.Metrics(rec)

	e := echo.New()
	e.Use(mw)
	e.GET("/error", func(c echo.Context) error {
		return errors.New("something went wrong")
	})

	req := httptest.NewRequest(http.MethodGet, "/error", nil)
	resp := httptest.NewRecorder()
	e.ServeHTTP(resp, req)

	assert.Equal(t, "something went wrong", capturedMetric.Error)
}

func TestMetrics_HTTPError(t *testing.T) {
	rec := mocks.NewMockHTTPRecorder(t)

	var capturedMetric metrics.HTTPMetric
	rec.EXPECT().RecordHTTP(mock.Anything).
		Run(func(m metrics.HTTPMetric) {
			capturedMetric = m
		}).Return().Once()

	mw := middleware.Metrics(rec)

	e := echo.New()
	e.Use(mw)
	e.GET("/http-error", func(c echo.Context) error {
		return echo.NewHTTPError(http.StatusNotFound, "not found")
	})

	req := httptest.NewRequest(http.MethodGet, "/http-error", nil)
	resp := httptest.NewRecorder()
	e.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusNotFound, capturedMetric.StatusCode)
}

func TestMetrics_DurationInMilliseconds(t *testing.T) {
	rec := mocks.NewMockHTTPRecorder(t)

	var capturedMetric metrics.HTTPMetric
	rec.EXPECT().RecordHTTP(mock.Anything).
		Run(func(m metrics.HTTPMetric) {
			capturedMetric = m
		}).Return().Once()

	mw := middleware.Metrics(rec)

	e := echo.New()
	e.Use(mw)
	e.GET("/test", func(c echo.Context) error {
		return c.String(http.StatusOK, "ok")
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	resp := httptest.NewRecorder()
	e.ServeHTTP(resp, req)

	assert.GreaterOrEqual(t, capturedMetric.DurationMs, 0.0)
	assert.LessOrEqual(t, capturedMetric.DurationMs, 1000.0) // Sanity check: should take less than 1 second
}

func TestMetrics_DifferentMethods(t *testing.T) {
	methods := []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodDelete}

	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			rec := mocks.NewMockHTTPRecorder(t)

			var capturedMetric metrics.HTTPMetric
			rec.EXPECT().RecordHTTP(mock.Anything).
				Run(func(m metrics.HTTPMetric) {
					capturedMetric = m
				}).Return().Once()

			mw := middleware.Metrics(rec)

			e := echo.New()
			e.Use(mw)
			e.Add(method, "/test", func(c echo.Context) error {
				return c.String(http.StatusOK, "ok")
			})

			req := httptest.NewRequest(method, "/test", nil)
			resp := httptest.NewRecorder()
			e.ServeHTTP(resp, req)

			require.NotZero(t, capturedMetric.Method)
			assert.Equal(t, method, capturedMetric.Method)
		})
	}
}

func TestMetrics_PathParameter(t *testing.T) {
	rec := mocks.NewMockHTTPRecorder(t)

	var capturedMetric metrics.HTTPMetric
	rec.EXPECT().RecordHTTP(mock.Anything).
		Run(func(m metrics.HTTPMetric) {
			capturedMetric = m
		}).Return().Once()

	mw := middleware.Metrics(rec)

	e := echo.New()
	e.Use(mw)
	e.GET("/:code", func(c echo.Context) error {
		return c.String(http.StatusOK, "ok")
	})

	req := httptest.NewRequest(http.MethodGet, "/abc123", nil)
	resp := httptest.NewRecorder()
	e.ServeHTTP(resp, req)

	// Path should be the template, not the actual value
	assert.Equal(t, "/:code", capturedMetric.Path)
}
