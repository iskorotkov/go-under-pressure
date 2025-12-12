package seed

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"runtime"
	"sync/atomic"
	"time"

	"golang.org/x/sync/errgroup"
)

type batchRequest struct {
	URLs []string `json:"urls"`
}

type createResponse struct {
	ShortCode string `json:"short_code"`
}

type batchResponse struct {
	URLs []createResponse `json:"urls"`
}

const bypassHeader = "X-Rate-Limit-Bypass"

func Run(baseURL string, count, batchSize int, bypassSecret string, insecureSkipVerify bool, timeout time.Duration) ([]string, error) {
	numWorkers := runtime.NumCPU() * 2
	fmt.Printf("Seeding %d URLs (batch size: %d, workers: %d)...\n", count, batchSize, numWorkers)

	client := &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			TLSClientConfig:     &tls.Config{InsecureSkipVerify: insecureSkipVerify},
			MaxIdleConns:        numWorkers * 2,
			MaxIdleConnsPerHost: numWorkers * 2,
			IdleConnTimeout:     90 * time.Second,
			ForceAttemptHTTP2:   true,
		},
	}

	numBatches := (count + batchSize - 1) / batchSize
	results := make([][]string, numBatches)
	var progress atomic.Int64

	g, _ := errgroup.WithContext(context.Background())
	g.SetLimit(numWorkers)

	for i := range numBatches {
		batchIndex := i
		startIndex := batchIndex * batchSize
		currentBatch := min(batchSize, count-startIndex)

		g.Go(func() error {
			batchCodes, err := createBatch(client, baseURL, startIndex, currentBatch, bypassSecret)
			if err != nil {
				return fmt.Errorf("failed to create batch at %d: %w", startIndex, err)
			}
			results[batchIndex] = batchCodes
			done := progress.Add(int64(len(batchCodes)))
			fmt.Printf("\rProgress: %d/%d", done, count)
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return nil, err
	}

	codes := make([]string, 0, count)
	for _, batch := range results {
		codes = append(codes, batch...)
	}

	fmt.Printf("\nSeeding complete: %d codes\n", len(codes))
	return codes, nil
}

func createBatch(client *http.Client, baseURL string, startIndex, count int, bypassSecret string) ([]string, error) {
	urls := make([]string, count)
	for i := range count {
		urls[i] = fmt.Sprintf("https://example.com/seed/%d", startIndex+i)
	}

	body, err := json.Marshal(batchRequest{URLs: urls})
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest(http.MethodPost, baseURL+"/api/v1/urls/batch", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if bypassSecret != "" {
		req.Header.Set(bypassHeader, bypassSecret)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}

	var result batchResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	codes := make([]string, len(result.URLs))
	for i, u := range result.URLs {
		codes[i] = u.ShortCode
	}
	return codes, nil
}
