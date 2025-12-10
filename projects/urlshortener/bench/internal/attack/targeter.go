package attack

import (
	"fmt"
	"math/rand/v2"
	"net/http"
	"time"

	vegeta "github.com/tsenart/vegeta/v12/lib"
)

const bypassHeader = "X-Rate-Limit-Bypass"

func CreateTargeter(baseURL, bypassSecret string) vegeta.Targeter {
	header := http.Header{"Content-Type": []string{"application/json"}}
	if bypassSecret != "" {
		header.Set(bypassHeader, bypassSecret)
	}

	return func(t *vegeta.Target) error {
		t.Method = http.MethodPost
		t.URL = baseURL + "/api/v1/urls"
		t.Header = header
		t.Body = []byte(fmt.Sprintf(`{"url":"https://example.com/%d/%d"}`,
			time.Now().UnixNano(), rand.Int()))
		return nil
	}
}

func RedirectTargeter(baseURL string, codes []string, bypassSecret string) vegeta.Targeter {
	var header http.Header
	if bypassSecret != "" {
		header = http.Header{bypassHeader: []string{bypassSecret}}
	}

	return func(t *vegeta.Target) error {
		code := codes[rand.IntN(len(codes))]
		t.Method = http.MethodGet
		t.URL = baseURL + "/" + code
		t.Header = header
		return nil
	}
}

func MixedTargeter(baseURL string, codes []string, createRatio float64, bypassSecret string) vegeta.Targeter {
	createTarget := CreateTargeter(baseURL, bypassSecret)
	redirectTarget := RedirectTargeter(baseURL, codes, bypassSecret)

	return func(t *vegeta.Target) error {
		if rand.Float64() < createRatio {
			return createTarget(t)
		}
		return redirectTarget(t)
	}
}
