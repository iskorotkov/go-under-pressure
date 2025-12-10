package validation_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"urlshortener/internal/validation"
)

func TestIPValidator_ValidateHost(t *testing.T) {
	v := validation.NewIPValidator()

	tests := []struct {
		name    string
		host    string
		wantErr error
	}{
		// Hostnames (no DNS resolution, so allowed)
		{"hostname", "example.com", nil},
		{"hostname with port", "example.com:8080", nil},
		{"localhost hostname", "localhost", nil},
		{"localhost hostname with port", "localhost:8080", nil},

		// Public IPs
		{"public ipv4", "8.8.8.8", nil},
		{"public ipv4 with port", "8.8.8.8:80", nil},
		{"public ipv6", "[2001:4860:4860::8888]", nil},

		// Loopback
		{"loopback ipv4", "127.0.0.1", validation.ErrPrivateIPNotAllowed},
		{"loopback ipv4 with port", "127.0.0.1:8080", validation.ErrPrivateIPNotAllowed},
		{"loopback ipv6", "[::1]", validation.ErrPrivateIPNotAllowed},

		// Private ranges
		{"private 10.x", "10.0.0.1", validation.ErrPrivateIPNotAllowed},
		{"private 172.16.x", "172.16.0.1", validation.ErrPrivateIPNotAllowed},
		{"private 172.31.x", "172.31.255.255", validation.ErrPrivateIPNotAllowed},
		{"private 192.168.x", "192.168.1.1", validation.ErrPrivateIPNotAllowed},

		// Link-local
		{"link-local ipv4", "169.254.1.1", validation.ErrPrivateIPNotAllowed},

		// CGNAT
		{"cgnat", "100.64.0.1", validation.ErrPrivateIPNotAllowed},
		{"cgnat upper", "100.127.255.255", validation.ErrPrivateIPNotAllowed},

		// Documentation ranges
		{"test-net-1", "192.0.2.1", validation.ErrPrivateIPNotAllowed},
		{"test-net-2", "198.51.100.1", validation.ErrPrivateIPNotAllowed},
		{"test-net-3", "203.0.113.1", validation.ErrPrivateIPNotAllowed},

		// IETF Protocol Assignments
		{"ietf protocol", "192.0.0.1", validation.ErrPrivateIPNotAllowed},

		// Unspecified
		{"unspecified ipv4", "0.0.0.0", validation.ErrPrivateIPNotAllowed},
		{"unspecified ipv6", "[::]", validation.ErrPrivateIPNotAllowed},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := v.ValidateHost(tt.host)
			if tt.wantErr != nil {
				assert.ErrorIs(t, err, tt.wantErr)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestIPValidator_ValidateHost_IPv4MappedIPv6(t *testing.T) {
	v := validation.NewIPValidator()

	tests := []struct {
		name    string
		host    string
		wantErr error
	}{
		{"ipv4-mapped public", "[::ffff:8.8.8.8]", nil},
		{"ipv4-mapped loopback", "[::ffff:127.0.0.1]", validation.ErrPrivateIPNotAllowed},
		{"ipv4-mapped private", "[::ffff:192.168.1.1]", validation.ErrPrivateIPNotAllowed},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := v.ValidateHost(tt.host)
			if tt.wantErr != nil {
				assert.ErrorIs(t, err, tt.wantErr)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
