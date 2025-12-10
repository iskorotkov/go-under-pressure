package validation

import (
	"net/netip"
	"strings"
)

type IPValidator struct{}

func NewIPValidator() *IPValidator {
	return &IPValidator{}
}

func (v *IPValidator) ValidateHost(host string) error {
	hostname := host
	if idx := strings.LastIndex(host, ":"); idx != -1 {
		// Check if this is IPv6 with brackets
		if !strings.HasPrefix(host, "[") {
			hostname = host[:idx]
		}
	}

	// Handle IPv6 brackets
	hostname = strings.TrimPrefix(hostname, "[")
	hostname = strings.TrimSuffix(hostname, "]")
	if idx := strings.LastIndex(hostname, "]:"); idx != -1 {
		hostname = hostname[:idx]
	}

	addr, err := netip.ParseAddr(hostname)
	if err != nil {
		// Not an IP literal, skip validation (no DNS resolution per design decision)
		return nil
	}

	return v.validateIP(addr)
}

func (v *IPValidator) validateIP(addr netip.Addr) error {
	// Handle IPv4-mapped IPv6 addresses
	if addr.Is4In6() {
		addr = netip.AddrFrom4(addr.As4())
	}

	if addr.IsPrivate() ||
		addr.IsLoopback() ||
		addr.IsLinkLocalUnicast() ||
		addr.IsLinkLocalMulticast() ||
		addr.IsMulticast() ||
		addr.IsUnspecified() {
		return ErrPrivateIPNotAllowed
	}

	if v.isReservedRange(addr) {
		return ErrPrivateIPNotAllowed
	}

	return nil
}

func (v *IPValidator) isReservedRange(addr netip.Addr) bool {
	if !addr.Is4() {
		return false
	}

	ip4 := addr.As4()

	// 100.64.0.0/10 (Carrier-grade NAT)
	if ip4[0] == 100 && ip4[1] >= 64 && ip4[1] <= 127 {
		return true
	}

	// 192.0.0.0/24 (IETF Protocol Assignments)
	if ip4[0] == 192 && ip4[1] == 0 && ip4[2] == 0 {
		return true
	}

	// 192.0.2.0/24 (TEST-NET-1)
	if ip4[0] == 192 && ip4[1] == 0 && ip4[2] == 2 {
		return true
	}

	// 198.51.100.0/24 (TEST-NET-2)
	if ip4[0] == 198 && ip4[1] == 51 && ip4[2] == 100 {
		return true
	}

	// 203.0.113.0/24 (TEST-NET-3)
	if ip4[0] == 203 && ip4[1] == 0 && ip4[2] == 113 {
		return true
	}

	return false
}
