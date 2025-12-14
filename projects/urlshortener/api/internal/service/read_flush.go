package service

import (
	"context"
	"fmt"
	"time"

	"urlshortener/internal/batcher"
)

type LookupRequest struct {
	ShortCode string
}

func (s *URLService) LookupFlushFunc(ctx context.Context, batch []batcher.Request[LookupRequest, string]) {
	if len(batch) == 0 {
		return
	}

	shortCodes := make([]string, len(batch))
	requestMap := make(map[string][]batcher.Request[LookupRequest, string], len(batch))

	for i, req := range batch {
		shortCodes[i] = req.Input.ShortCode
		requestMap[req.Input.ShortCode] = append(requestMap[req.Input.ShortCode], req)
	}

	results, err := s.repo.FindByShortCodes(ctx, shortCodes)
	if err != nil {
		sendErrorToAll(batch, fmt.Errorf("failed to find urls: %w", err))
		return
	}

	now := time.Now()
	foundCount := 0
	notFoundCount := 0

	for shortCode, requests := range requestMap {
		originalURL, found := results[shortCode]
		if found {
			s.cache.Set(shortCode, originalURL)
			for _, req := range requests {
				if req.ResultCh != nil {
					req.ResultCh <- batcher.Result[string]{Value: originalURL}
				}
			}
			foundCount++
		} else {
			for _, req := range requests {
				if req.ResultCh != nil {
					req.ResultCh <- batcher.Result[string]{Err: ErrURLNotFound}
				}
			}
			notFoundCount++
		}
	}

	s.recorder.RecordBusiness(now, "read_batcher_flush_size", float64(len(batch)), nil)
	if foundCount > 0 {
		s.recorder.RecordBusiness(now, "read_batcher_found", float64(foundCount), nil)
	}
	if notFoundCount > 0 {
		s.recorder.RecordBusiness(now, "read_batcher_not_found", float64(notFoundCount), nil)
	}
}
