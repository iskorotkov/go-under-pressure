package validation

import (
	"net/url"
	"strings"
)

var blockedProtocols = map[string]bool{
	"javascript": true,
	"data":       true,
	"file":       true,
	"vbscript":   true,
	"about":      true,
	"blob":       true,
}

var allowedProtocols = map[string]bool{
	"http":  true,
	"https": true,
}

type URLValidator struct {
	maxLength       int
	maxBatchSize    int
	allowPrivateIPs bool
	ipValidator     *IPValidator
}

func NewURLValidator(maxLength, maxBatchSize int, allowPrivateIPs bool) *URLValidator {
	return &URLValidator{
		maxLength:       maxLength,
		maxBatchSize:    maxBatchSize,
		allowPrivateIPs: allowPrivateIPs,
		ipValidator:     NewIPValidator(),
	}
}

func (v *URLValidator) ValidateURL(rawURL string) error {
	if strings.TrimSpace(rawURL) == "" {
		return ErrEmptyURL
	}

	if len(rawURL) > v.maxLength {
		return ErrURLTooLong
	}

	parsed, err := url.Parse(rawURL)
	if err != nil {
		return ErrInvalidURLFormat
	}

	scheme := strings.ToLower(parsed.Scheme)
	if blockedProtocols[scheme] {
		return ErrUnsafeProtocol
	}
	if !allowedProtocols[scheme] {
		return ErrInvalidURLFormat
	}

	if parsed.Host == "" {
		return ErrInvalidURLFormat
	}

	if !v.allowPrivateIPs {
		if err := v.ipValidator.ValidateHost(parsed.Host); err != nil {
			return err
		}
	}

	return nil
}

func (v *URLValidator) ValidateBatch(urls []string) error {
	if len(urls) == 0 {
		return ErrEmptyBatch
	}

	if len(urls) > v.maxBatchSize {
		return ErrBatchTooLarge
	}

	var batchErrors []IndexedError
	for i, u := range urls {
		if err := v.ValidateURL(u); err != nil {
			batchErrors = append(batchErrors, IndexedError{Index: i, Err: err})
		}
	}

	if len(batchErrors) > 0 {
		return &BatchValidationError{Errors: batchErrors}
	}

	return nil
}
