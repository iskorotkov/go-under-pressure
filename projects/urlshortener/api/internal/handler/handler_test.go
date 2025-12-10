package handler

import "testing"

func TestExtractDomain(t *testing.T) {
	tests := []struct {
		name     string
		referer  string
		expected string
	}{
		{
			name:     "empty referer returns direct",
			referer:  "",
			expected: "direct",
		},
		{
			name:     "https url extracts host",
			referer:  "https://google.com/search?q=test",
			expected: "google.com",
		},
		{
			name:     "http url extracts host",
			referer:  "http://example.com/path",
			expected: "example.com",
		},
		{
			name:     "url with port preserves port",
			referer:  "http://example.com:8080/path",
			expected: "example.com:8080",
		},
		{
			name:     "subdomain preserved",
			referer:  "https://sub.domain.com/",
			expected: "sub.domain.com",
		},
		{
			name:     "invalid url returns unknown",
			referer:  "not-a-valid-url",
			expected: "unknown",
		},
		{
			name:     "url without host returns unknown",
			referer:  "/just/a/path",
			expected: "unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractDomain(tt.referer)
			if result != tt.expected {
				t.Errorf("extractDomain(%q) = %q, want %q", tt.referer, result, tt.expected)
			}
		})
	}
}
