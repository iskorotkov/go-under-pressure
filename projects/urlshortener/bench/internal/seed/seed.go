package seed

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
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
	fmt.Printf("Seeding %d URLs (batch size: %d)...\n", count, batchSize)

	codes := make([]string, 0, count)
	client := &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: insecureSkipVerify},
		},
	}

	for i := 0; i < count; i += batchSize {
		currentBatch := min(batchSize, count-i)
		batchCodes, err := createBatch(client, baseURL, i, currentBatch, bypassSecret)
		if err != nil {
			return nil, fmt.Errorf("failed to create batch at %d: %w", i, err)
		}
		codes = append(codes, batchCodes...)
		fmt.Printf("\rProgress: %d/%d", len(codes), count)
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
