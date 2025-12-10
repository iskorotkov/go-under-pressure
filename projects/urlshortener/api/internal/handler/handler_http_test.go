package handler_test

import (
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"urlshortener/internal/domain"
	"urlshortener/internal/handler"
	"urlshortener/internal/handler/mocks"
	"urlshortener/internal/service"
	"urlshortener/internal/validation"
)

func newTestHandler(t *testing.T) (*handler.Handler, *mocks.MockURLService, *mocks.MockURLValidator, *mocks.MockBusinessRecorder) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	svc := mocks.NewMockURLService(t)
	val := mocks.NewMockURLValidator(t)
	rec := mocks.NewMockBusinessRecorder(t)
	h := handler.New(svc, val, logger, rec)
	return h, svc, val, rec
}

// CreateURL tests

func TestCreateURL_Success(t *testing.T) {
	h, svc, val, _ := newTestHandler(t)

	val.EXPECT().ValidateURL("https://example.com").Return(nil)
	svc.EXPECT().CreateShortURL(mock.Anything, "https://example.com").Return(&domain.CreateURLResponse{
		ShortCode:   "xyz789",
		ShortURL:    "http://short.url/xyz789",
		OriginalURL: "https://example.com",
	}, nil)

	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/urls", strings.NewReader(`{"url":"https://example.com"}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := h.CreateURL(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusCreated, rec.Code)
	assert.Contains(t, rec.Body.String(), "xyz789")
}

func TestCreateURL_InvalidJSON(t *testing.T) {
	h, _, _, _ := newTestHandler(t)

	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/urls", strings.NewReader(`invalid json`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := h.CreateURL(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestCreateURL_EmptyURL(t *testing.T) {
	h, _, val, _ := newTestHandler(t)

	val.EXPECT().ValidateURL("").Return(validation.ErrEmptyURL)

	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/urls", strings.NewReader(`{"url":""}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := h.CreateURL(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "url is required")
}

func TestCreateURL_InvalidURLFormat(t *testing.T) {
	h, _, val, _ := newTestHandler(t)

	val.EXPECT().ValidateURL("not-a-url").Return(validation.ErrInvalidURLFormat)

	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/urls", strings.NewReader(`{"url":"not-a-url"}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := h.CreateURL(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "invalid url format")
}

func TestCreateURL_ServiceError(t *testing.T) {
	h, svc, val, _ := newTestHandler(t)

	val.EXPECT().ValidateURL("https://example.com").Return(nil)
	svc.EXPECT().CreateShortURL(mock.Anything, "https://example.com").Return(nil, errors.New("db error"))

	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/urls", strings.NewReader(`{"url":"https://example.com"}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := h.CreateURL(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

// CreateURLBatch tests

func TestCreateURLBatch_Success(t *testing.T) {
	h, svc, val, _ := newTestHandler(t)

	val.EXPECT().ValidateBatch([]string{"https://example.com/1", "https://example.com/2"}).Return(nil)
	svc.EXPECT().CreateShortURLBatch(mock.Anything, []string{"https://example.com/1", "https://example.com/2"}).
		Return([]domain.CreateURLResponse{
			{ShortCode: "code0", ShortURL: "http://short.url/code0", OriginalURL: "https://example.com/1"},
			{ShortCode: "code1", ShortURL: "http://short.url/code1", OriginalURL: "https://example.com/2"},
		}, nil)

	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/urls/batch",
		strings.NewReader(`{"urls":["https://example.com/1","https://example.com/2"]}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := h.CreateURLBatch(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusCreated, rec.Code)
}

func TestCreateURLBatch_EmptyBatch(t *testing.T) {
	h, _, val, _ := newTestHandler(t)

	val.EXPECT().ValidateBatch([]string{}).Return(validation.ErrEmptyBatch)

	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/urls/batch", strings.NewReader(`{"urls":[]}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := h.CreateURLBatch(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "urls is required")
}

func TestCreateURLBatch_TooLarge(t *testing.T) {
	h, _, val, _ := newTestHandler(t)

	val.EXPECT().ValidateBatch([]string{"url1", "url2"}).Return(validation.ErrBatchTooLarge)

	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/urls/batch", strings.NewReader(`{"urls":["url1","url2"]}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := h.CreateURLBatch(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "batch size exceeds maximum")
}

func TestCreateURLBatch_BatchValidationError(t *testing.T) {
	h, _, val, _ := newTestHandler(t)

	val.EXPECT().ValidateBatch([]string{"https://example.com", "javascript:alert(1)"}).
		Return(&validation.BatchValidationError{
			Errors: []validation.IndexedError{
				{Index: 1, Err: validation.ErrUnsafeProtocol},
			},
		})

	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/urls/batch",
		strings.NewReader(`{"urls":["https://example.com","javascript:alert(1)"]}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := h.CreateURLBatch(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "errors")
}

func TestCreateURLBatch_ServiceError(t *testing.T) {
	h, svc, val, _ := newTestHandler(t)

	val.EXPECT().ValidateBatch([]string{"https://example.com"}).Return(nil)
	svc.EXPECT().CreateShortURLBatch(mock.Anything, []string{"https://example.com"}).
		Return(nil, errors.New("batch error"))

	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/urls/batch", strings.NewReader(`{"urls":["https://example.com"]}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := h.CreateURLBatch(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

// Redirect tests

func TestRedirect_Success(t *testing.T) {
	h, svc, _, recorder := newTestHandler(t)

	svc.EXPECT().GetOriginalURL(mock.Anything, "abc123").Return("https://example.com/redirect-target", nil)
	recorder.EXPECT().RecordBusiness("unique_visitors", float64(1), mock.Anything).Return()
	recorder.EXPECT().RecordBusiness("referrer_redirects", float64(1), mock.Anything).Return()

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/abc123", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetPath("/:code")
	c.SetParamNames("code")
	c.SetParamValues("abc123")

	err := h.Redirect(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusFound, rec.Code)
	assert.Equal(t, "https://example.com/redirect-target", rec.Header().Get("Location"))
}

func TestRedirect_EmptyCode(t *testing.T) {
	h, _, _, _ := newTestHandler(t)

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetPath("/:code")
	c.SetParamNames("code")
	c.SetParamValues("")

	err := h.Redirect(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestRedirect_NotFound(t *testing.T) {
	h, svc, _, recorder := newTestHandler(t)

	svc.EXPECT().GetOriginalURL(mock.Anything, "notfound").Return("", service.ErrURLNotFound)
	recorder.EXPECT().RecordBusiness("url_not_found", float64(1), mock.Anything).Return()

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/notfound", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetPath("/:code")
	c.SetParamNames("code")
	c.SetParamValues("notfound")

	err := h.Redirect(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestRedirect_ServiceError(t *testing.T) {
	h, svc, _, _ := newTestHandler(t)

	svc.EXPECT().GetOriginalURL(mock.Anything, "abc123").Return("", errors.New("db error"))

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/abc123", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetPath("/:code")
	c.SetParamNames("code")
	c.SetParamValues("abc123")

	err := h.Redirect(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

// handleValidationError tests

func TestHandleValidationError_AllErrorTypes(t *testing.T) {
	tests := []struct {
		name        string
		err         error
		wantStatus  int
		wantMessage string
	}{
		{"ErrEmptyURL", validation.ErrEmptyURL, http.StatusBadRequest, "url is required"},
		{"ErrInvalidURLFormat", validation.ErrInvalidURLFormat, http.StatusBadRequest, "invalid url format"},
		{"ErrUnsafeProtocol", validation.ErrUnsafeProtocol, http.StatusBadRequest, "url protocol not allowed"},
		{"ErrURLTooLong", validation.ErrURLTooLong, http.StatusBadRequest, "url exceeds maximum length"},
		{"ErrPrivateIPNotAllowed", validation.ErrPrivateIPNotAllowed, http.StatusBadRequest, "private ip addresses not allowed"},
		{"ErrBatchTooLarge", validation.ErrBatchTooLarge, http.StatusBadRequest, "batch size exceeds maximum"},
		{"ErrEmptyBatch", validation.ErrEmptyBatch, http.StatusBadRequest, "urls is required"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h, _, val, _ := newTestHandler(t)

			val.EXPECT().ValidateURL("test").Return(tt.err)

			e := echo.New()
			req := httptest.NewRequest(http.MethodPost, "/api/v1/urls", strings.NewReader(`{"url":"test"}`))
			req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
			rec := httptest.NewRecorder()
			c := e.NewContext(req, rec)

			err := h.CreateURL(c)
			require.NoError(t, err)
			assert.Equal(t, tt.wantStatus, rec.Code)
			assert.Contains(t, rec.Body.String(), tt.wantMessage)
		})
	}
}

// Health endpoint test

func TestHealth(t *testing.T) {
	h, _, _, _ := newTestHandler(t)

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := h.Health(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "ok")
}

// InvalidJSON for batch
func TestCreateURLBatch_InvalidJSON(t *testing.T) {
	h, _, _, _ := newTestHandler(t)

	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/urls/batch", strings.NewReader(`invalid json`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := h.CreateURLBatch(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}
