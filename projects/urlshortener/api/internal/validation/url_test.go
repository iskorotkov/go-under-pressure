package validation_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"urlshortener/internal/validation"
)

func TestURLValidator_ValidateURL(t *testing.T) {
	v := validation.NewURLValidator(2048, 100, false)

	tests := []struct {
		name    string
		url     string
		wantErr error
	}{
		// Valid URLs
		{"valid http", "http://example.com", nil},
		{"valid https", "https://example.com", nil},
		{"valid with path", "https://example.com/path", nil},
		{"valid with query", "https://example.com/path?q=1", nil},
		{"valid with fragment", "https://example.com/path#section", nil},
		{"valid with port", "https://example.com:8080/path", nil},

		// Empty/missing
		{"empty string", "", validation.ErrEmptyURL},
		{"whitespace only", "   ", validation.ErrEmptyURL},

		// Invalid format
		{"no scheme", "example.com", validation.ErrInvalidURLFormat},
		{"no host", "http://", validation.ErrInvalidURLFormat},
		{"ftp scheme", "ftp://example.com", validation.ErrInvalidURLFormat},

		// Blocked protocols
		{"javascript protocol", "javascript:alert(1)", validation.ErrUnsafeProtocol},
		{"data protocol", "data:text/html,<script>", validation.ErrUnsafeProtocol},
		{"file protocol", "file:///etc/passwd", validation.ErrUnsafeProtocol},
		{"vbscript protocol", "vbscript:msgbox(1)", validation.ErrUnsafeProtocol},
		{"about protocol", "about:blank", validation.ErrUnsafeProtocol},
		{"blob protocol", "blob:http://example.com/uuid", validation.ErrUnsafeProtocol},

		// Private IPs
		{"localhost", "http://127.0.0.1/", validation.ErrPrivateIPNotAllowed},
		{"loopback", "http://127.0.0.1/path", validation.ErrPrivateIPNotAllowed},
		{"private 10.x", "http://10.0.0.1/", validation.ErrPrivateIPNotAllowed},
		{"private 172.16.x", "http://172.16.0.1/", validation.ErrPrivateIPNotAllowed},
		{"private 192.168.x", "http://192.168.1.1/", validation.ErrPrivateIPNotAllowed},
		{"ipv6 loopback", "http://[::1]/", validation.ErrPrivateIPNotAllowed},

		// Hostnames are allowed (no DNS resolution)
		{"localhost hostname", "http://localhost/", nil},
		{"internal hostname", "http://internal-server/", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := v.ValidateURL(tt.url)
			if tt.wantErr != nil {
				assert.ErrorIs(t, err, tt.wantErr)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestURLValidator_ValidateURL_Length(t *testing.T) {
	v := validation.NewURLValidator(100, 100, false)

	shortURL := "https://example.com"
	err := v.ValidateURL(shortURL)
	assert.NoError(t, err)

	longURL := "https://example.com/" + strings.Repeat("a", 100)
	err = v.ValidateURL(longURL)
	assert.ErrorIs(t, err, validation.ErrURLTooLong)
}

func TestURLValidator_ValidateURL_AllowPrivateIPs(t *testing.T) {
	v := validation.NewURLValidator(2048, 100, true)

	privateIPs := []string{
		"http://127.0.0.1/",
		"http://10.0.0.1/",
		"http://192.168.1.1/",
		"http://[::1]/",
	}

	for _, url := range privateIPs {
		err := v.ValidateURL(url)
		assert.NoError(t, err, "URL %q should be allowed with allowPrivateIPs=true", url)
	}
}

func TestURLValidator_ValidateBatch(t *testing.T) {
	v := validation.NewURLValidator(2048, 3, false)

	t.Run("empty batch", func(t *testing.T) {
		err := v.ValidateBatch([]string{})
		assert.ErrorIs(t, err, validation.ErrEmptyBatch)
	})

	t.Run("batch too large", func(t *testing.T) {
		urls := []string{
			"https://example.com/1",
			"https://example.com/2",
			"https://example.com/3",
			"https://example.com/4",
		}
		err := v.ValidateBatch(urls)
		assert.ErrorIs(t, err, validation.ErrBatchTooLarge)
	})

	t.Run("valid batch", func(t *testing.T) {
		urls := []string{
			"https://example.com/1",
			"https://example.com/2",
			"https://example.com/3",
		}
		err := v.ValidateBatch(urls)
		assert.NoError(t, err)
	})

	t.Run("batch with invalid urls", func(t *testing.T) {
		urls := []string{
			"https://example.com/1",
			"javascript:alert(1)",
			"https://example.com/3",
		}
		err := v.ValidateBatch(urls)
		batchErr, ok := err.(*validation.BatchValidationError)
		require.True(t, ok, "expected *BatchValidationError, got %T", err)
		require.Len(t, batchErr.Errors, 1)
		assert.Equal(t, 1, batchErr.Errors[0].Index)
	})
}
