package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"

	"urlshortener/internal/middleware"
)

func TestPprofAuth(t *testing.T) {
	tests := []struct {
		name           string
		secret         string
		headerValue    string
		expectedStatus int
	}{
		{
			name:           "empty secret allows all",
			secret:         "",
			headerValue:    "",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "valid secret",
			secret:         "test-secret",
			headerValue:    "test-secret",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "invalid secret",
			secret:         "test-secret",
			headerValue:    "wrong",
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:           "missing header with secret configured",
			secret:         "test-secret",
			headerValue:    "",
			expectedStatus: http.StatusUnauthorized,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := echo.New()
			e.Use(middleware.PprofAuth(tt.secret))
			e.GET("/debug/pprof/", func(c echo.Context) error {
				return c.String(http.StatusOK, "ok")
			})

			req := httptest.NewRequest(http.MethodGet, "/debug/pprof/", nil)
			if tt.headerValue != "" {
				req.Header.Set("X-Pprof-Secret", tt.headerValue)
			}
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, req)

			assert.Equal(t, tt.expectedStatus, rec.Code)
		})
	}
}

func TestPprofAuth_ConstantTimeComparison(t *testing.T) {
	e := echo.New()
	e.Use(middleware.PprofAuth("secret123"))
	e.GET("/test", func(c echo.Context) error {
		return c.String(http.StatusOK, "ok")
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-Pprof-Secret", "secret12")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestRegisterPprof(t *testing.T) {
	e := echo.New()
	g := e.Group("/debug/pprof")
	middleware.RegisterPprof(g)

	endpoints := []string{
		"/debug/pprof/",
		"/debug/pprof/heap",
		"/debug/pprof/goroutine",
		"/debug/pprof/allocs",
		"/debug/pprof/block",
		"/debug/pprof/mutex",
		"/debug/pprof/threadcreate",
		"/debug/pprof/cmdline",
	}

	for _, ep := range endpoints {
		t.Run(ep, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, ep, nil)
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, req)

			assert.Equal(t, http.StatusOK, rec.Code, "endpoint %s should respond", ep)
		})
	}
}
