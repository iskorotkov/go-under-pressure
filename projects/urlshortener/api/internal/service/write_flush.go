package service

import (
	"context"
	"fmt"
	"time"

	"urlshortener/internal/batcher"
	"urlshortener/internal/domain"
	"urlshortener/internal/repository"
)

type CreateRequest struct {
	OriginalURL string
}

func (s *URLService) CreateFlushFunc(ctx context.Context, batch []batcher.Request[CreateRequest, *domain.CreateURLResponse]) {
	if len(batch) == 0 {
		return
	}

	ids, err := s.repo.NextIDs(ctx, len(batch))
	if err != nil {
		sendErrorToAll(batch, fmt.Errorf("failed to get next ids: %w", err))
		return
	}

	urlRows := make([]repository.URLRow, 0, len(batch))
	responses := make([]*domain.CreateURLResponse, len(batch))
	failedIndices := make(map[int]bool)

	for i, req := range batch {
		shortCode, err := s.shortener.Generate(ids[i])
		if err != nil {
			if req.ResultCh != nil {
				req.ResultCh <- batcher.Result[*domain.CreateURLResponse]{
					Err: fmt.Errorf("failed to generate short code: %w", err),
				}
			}
			failedIndices[i] = true
			continue
		}

		urlRows = append(urlRows, repository.URLRow{
			ShortCode:   shortCode,
			OriginalURL: req.Input.OriginalURL,
		})

		responses[i] = &domain.CreateURLResponse{
			ShortCode:   shortCode,
			ShortURL:    s.baseURL + "/" + shortCode,
			OriginalURL: req.Input.OriginalURL,
		}
	}

	if len(urlRows) == 0 {
		return
	}

	if err := s.repo.CreateBatch(ctx, urlRows); err != nil {
		for i, req := range batch {
			if !failedIndices[i] && req.ResultCh != nil {
				req.ResultCh <- batcher.Result[*domain.CreateURLResponse]{
					Err: fmt.Errorf("failed to create urls: %w", err),
				}
			}
		}
		return
	}

	successCount := 0
	for i, req := range batch {
		if failedIndices[i] {
			continue
		}
		s.cache.Set(responses[i].ShortCode, req.Input.OriginalURL)
		if req.ResultCh != nil {
			req.ResultCh <- batcher.Result[*domain.CreateURLResponse]{Value: responses[i]}
		}
		successCount++
	}

	now := time.Now()
	labels := []byte(`{"method":"batched"}`)
	s.recorder.RecordBusiness(now, "urls_created", float64(successCount), labels)
	s.recorder.RecordBusiness(now, "write_batcher_flush_size", float64(len(batch)), nil)
}

func sendErrorToAll[Req, Res any](batch []batcher.Request[Req, Res], err error) {
	for _, req := range batch {
		if req.ResultCh != nil {
			req.ResultCh <- batcher.Result[Res]{Err: err}
		}
	}
}
