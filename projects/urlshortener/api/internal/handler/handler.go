package handler

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/labstack/echo/v4"

	"urlshortener/internal/domain"
	"urlshortener/internal/service"
)

type Handler struct {
	urlService *service.URLService
	logger     *slog.Logger
}

func New(urlService *service.URLService, logger *slog.Logger) *Handler {
	return &Handler{
		urlService: urlService,
		logger:     logger,
	}
}

func (h *Handler) Register(e *echo.Echo) {
	api := e.Group("/api/v1")
	api.POST("/urls", h.CreateURL)
	e.GET("/:code", h.Redirect)
}

func (h *Handler) CreateURL(c echo.Context) error {
	var req domain.CreateURLRequest
	if err := c.Bind(&req); err != nil {
		h.logger.Error("failed to bind request", slog.String("error", err.Error()))
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request body"})
	}

	if req.URL == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "url is required"})
	}

	resp, err := h.urlService.CreateShortURL(c.Request().Context(), req.URL)
	if err != nil {
		h.logger.Error("failed to create short url", slog.String("error", err.Error()))
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to create short url"})
	}

	return c.JSON(http.StatusCreated, resp)
}

func (h *Handler) Redirect(c echo.Context) error {
	code := c.Param("code")
	if code == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "code is required"})
	}

	originalURL, err := h.urlService.GetOriginalURL(c.Request().Context(), code)
	if err != nil {
		if errors.Is(err, service.ErrURLNotFound) {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "url not found"})
		}
		h.logger.Error("failed to get original url", slog.String("error", err.Error()))
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to get url"})
	}

	return c.Redirect(http.StatusFound, originalURL)
}
