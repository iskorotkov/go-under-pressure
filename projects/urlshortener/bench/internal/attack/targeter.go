package attack

import (
	"fmt"
	"math/rand/v2"
	"net/http"
	"time"

	vegeta "github.com/tsenart/vegeta/v12/lib"
)

func CreateTargeter(baseURL string) vegeta.Targeter {
	return func(t *vegeta.Target) error {
		t.Method = http.MethodPost
		t.URL = baseURL + "/api/v1/urls"
		t.Header = http.Header{"Content-Type": []string{"application/json"}}
		t.Body = []byte(fmt.Sprintf(`{"url":"https://example.com/%d/%d"}`,
			time.Now().UnixNano(), rand.Int()))
		return nil
	}
}

func RedirectTargeter(baseURL string, codes []string) vegeta.Targeter {
	return func(t *vegeta.Target) error {
		code := codes[rand.IntN(len(codes))]
		t.Method = http.MethodGet
		t.URL = baseURL + "/" + code
		return nil
	}
}

func MixedTargeter(baseURL string, codes []string, createRatio float64) vegeta.Targeter {
	createTarget := CreateTargeter(baseURL)
	redirectTarget := RedirectTargeter(baseURL, codes)

	return func(t *vegeta.Target) error {
		if rand.Float64() < createRatio {
			return createTarget(t)
		}
		return redirectTarget(t)
	}
}
