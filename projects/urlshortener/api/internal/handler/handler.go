package handler

import (
	"errors"
	"log/slog"
	"net/http"
	"net/url"

	"github.com/labstack/echo/v4"

	"urlshortener/internal/domain"
	"urlshortener/internal/service"
	"urlshortener/internal/validation"
)

var (
	errInvalidBody       = map[string]string{"error": "invalid request body"}
	errURLRequired       = map[string]string{"error": "url is required"}
	errURLsRequired      = map[string]string{"error": "urls is required"}
	errCodeRequired      = map[string]string{"error": "code is required"}
	errURLNotFound       = map[string]string{"error": "url not found"}
	errCreateFailed      = map[string]string{"error": "failed to create short url"}
	errCreateBatchFailed = map[string]string{"error": "failed to create short urls"}
	errGetFailed         = map[string]string{"error": "failed to get url"}
	errInvalidURL        = map[string]string{"error": "invalid url format"}
	errUnsafeURL         = map[string]string{"error": "url protocol not allowed"}
	errURLTooLong        = map[string]string{"error": "url exceeds maximum length"}
	errPrivateIP         = map[string]string{"error": "private ip addresses not allowed"}
	errBatchTooLarge     = map[string]string{"error": "batch size exceeds maximum"}
	respHealthOK         = map[string]string{"status": "ok"}
)

type Handler struct {
	urlService   URLService
	urlValidator URLValidator
	logger       *slog.Logger
	recorder     BusinessRecorder
}

func New(
	urlService URLService,
	urlValidator URLValidator,
	logger *slog.Logger,
	recorder BusinessRecorder,
) *Handler {
	return &Handler{
		urlService:   urlService,
		urlValidator: urlValidator,
		logger:       logger,
		recorder:     recorder,
	}
}

func (h *Handler) Register(e *echo.Echo) {
	api := e.Group("/api/v1")
	api.GET("/health", h.Health)
	api.POST("/urls", h.CreateURL)
	api.POST("/urls/batch", h.CreateURLBatch)
	e.GET("/:code", h.Redirect)
}

func (h *Handler) Health(c echo.Context) error {
	return c.JSON(http.StatusOK, respHealthOK)
}

func (h *Handler) CreateURL(c echo.Context) error {
	var req domain.CreateURLRequest
	if err := c.Bind(&req); err != nil {
		h.logger.Error("failed to bind request", slog.String("error", err.Error()))
		return c.JSON(http.StatusBadRequest, errInvalidBody)
	}

	if err := h.urlValidator.ValidateURL(req.URL); err != nil {
		return h.handleValidationError(c, err)
	}

	resp, err := h.urlService.CreateShortURL(c.Request().Context(), req.URL)
	if err != nil {
		h.logger.Error("failed to create short url", slog.String("error", err.Error()))
		return c.JSON(http.StatusInternalServerError, errCreateFailed)
	}

	return c.JSON(http.StatusCreated, resp)
}

func (h *Handler) CreateURLBatch(c echo.Context) error {
	var req domain.CreateURLBatchRequest
	if err := c.Bind(&req); err != nil {
		h.logger.Error("failed to bind request", slog.String("error", err.Error()))
		return c.JSON(http.StatusBadRequest, errInvalidBody)
	}

	if err := h.urlValidator.ValidateBatch(req.URLs); err != nil {
		return h.handleValidationError(c, err)
	}

	responses, err := h.urlService.CreateShortURLBatch(c.Request().Context(), req.URLs)
	if err != nil {
		h.logger.Error("failed to create short urls", slog.String("error", err.Error()))
		return c.JSON(http.StatusInternalServerError, errCreateBatchFailed)
	}

	return c.JSON(http.StatusCreated, domain.CreateURLBatchResponse{URLs: responses})
}

func (h *Handler) Redirect(c echo.Context) error {
	code := c.Param("code")
	if code == "" {
		return c.JSON(http.StatusBadRequest, errCodeRequired)
	}

	clientIP := c.RealIP()
	referrer := extractDomain(c.Request().Referer())

	originalURL, err := h.urlService.GetOriginalURL(c.Request().Context(), code)
	if err != nil {
		if errors.Is(err, service.ErrURLNotFound) {
			h.recorder.RecordBusiness("url_not_found", 1, map[string]string{
				"short_code": code,
				"client_ip":  clientIP,
				"referrer":   referrer,
			})
			return c.JSON(http.StatusNotFound, errURLNotFound)
		}
		h.logger.Error("failed to get original url", slog.String("error", err.Error()))
		return c.JSON(http.StatusInternalServerError, errGetFailed)
	}

	h.recorder.RecordBusiness("unique_visitors", 1, map[string]string{
		"short_code": code,
		"client_ip":  clientIP,
	})
	h.recorder.RecordBusiness("referrer_redirects", 1, map[string]string{
		"short_code": code,
		"referrer":   referrer,
	})

	return c.Redirect(http.StatusFound, originalURL)
}

func extractDomain(referer string) string {
	if referer == "" {
		return "direct"
	}

	parsed, err := url.Parse(referer)
	if err != nil || parsed.Host == "" {
		return "unknown"
	}

	return parsed.Host
}

func (h *Handler) handleValidationError(c echo.Context, err error) error {
	switch {
	case errors.Is(err, validation.ErrEmptyURL):
		return c.JSON(http.StatusBadRequest, errURLRequired)
	case errors.Is(err, validation.ErrInvalidURLFormat):
		return c.JSON(http.StatusBadRequest, errInvalidURL)
	case errors.Is(err, validation.ErrUnsafeProtocol):
		return c.JSON(http.StatusBadRequest, errUnsafeURL)
	case errors.Is(err, validation.ErrURLTooLong):
		return c.JSON(http.StatusBadRequest, errURLTooLong)
	case errors.Is(err, validation.ErrPrivateIPNotAllowed):
		return c.JSON(http.StatusBadRequest, errPrivateIP)
	case errors.Is(err, validation.ErrBatchTooLarge):
		return c.JSON(http.StatusBadRequest, errBatchTooLarge)
	case errors.Is(err, validation.ErrEmptyBatch):
		return c.JSON(http.StatusBadRequest, errURLsRequired)
	default:
		var batchErr *validation.BatchValidationError
		if errors.As(err, &batchErr) {
			return c.JSON(http.StatusBadRequest, h.formatBatchErrors(batchErr))
		}
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "validation failed"})
	}
}

func (h *Handler) formatBatchErrors(err *validation.BatchValidationError) map[string]any {
	errs := make([]map[string]any, len(err.Errors))
	for i, e := range err.Errors {
		errs[i] = map[string]any{
			"index": e.Index,
			"error": e.Err.Error(),
		}
	}
	return map[string]any{"errors": errs}
}
