package attack

import (
	"fmt"
	"math/rand/v2"
	"net/http"
	"sync"
	"sync/atomic"

	vegeta "github.com/tsenart/vegeta/v12/lib"
)

const bypassHeader = "X-Rate-Limit-Bypass"

var (
	urlCounter atomic.Uint64
	bodyPool   = sync.Pool{
		New: func() any {
			return make([]byte, 0, 64)
		},
	}
)

func CreateTargeter(baseURL, bypassSecret string) vegeta.Targeter {
	header := http.Header{"Content-Type": []string{"application/json"}}
	if bypassSecret != "" {
		header.Set(bypassHeader, bypassSecret)
	}
	url := baseURL + "/api/v1/urls"

	return func(t *vegeta.Target) error {
		t.Method = http.MethodPost
		t.URL = url
		t.Header = header

		buf := bodyPool.Get().([]byte)[:0]
		buf = fmt.Appendf(buf, `{"url":"https://example.com/%d"}`, urlCounter.Add(1))
		t.Body = buf
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
