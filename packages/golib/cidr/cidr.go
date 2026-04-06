// Package cidr provides CIDR notation parsing and IP range
// checking. Used for IP allowlisting, network segmentation,
// and access control based on client IP addresses.
package cidr

import (
	"fmt"
	"net"
	"strings"
)

// Range represents a CIDR range.
type Range struct {
	network *net.IPNet
	raw     string
}

// Parse parses a CIDR notation string.
func Parse(cidr string) (*Range, error) {
	_, network, err := net.ParseCIDR(cidr)
	if err != nil {
		return nil, fmt.Errorf("cidr: invalid notation %q: %w", cidr, err)
	}
	return &Range{network: network, raw: cidr}, nil
}

// Contains checks if an IP address is within the range.
func (r *Range) Contains(ip string) bool {
	parsed := net.ParseIP(ip)
	if parsed == nil {
		return false
	}
	return r.network.Contains(parsed)
}

// String returns the CIDR notation.
func (r *Range) String() string {
	return r.raw
}

// Network returns the network address.
func (r *Range) Network() string {
	return r.network.IP.String()
}

// Mask returns the network mask.
func (r *Range) Mask() string {
	return net.IP(r.network.Mask).String()
}

// List manages a collection of CIDR ranges for allowlisting.
type List struct {
	ranges []*Range
}

// NewList creates a CIDR list from notation strings.
func NewList(cidrs ...string) (*List, error) {
	l := &List{}
	for _, c := range cidrs {
		r, err := Parse(c)
		if err != nil {
			return nil, err
		}
		l.ranges = append(l.ranges, r)
	}
	return l, nil
}

// Contains checks if an IP is in any of the ranges.
func (l *List) Contains(ip string) bool {
	for _, r := range l.ranges {
		if r.Contains(ip) {
			return true
		}
	}
	return false
}

// Len returns the number of ranges.
func (l *List) Len() int {
	return len(l.ranges)
}

// IsPrivate checks if an IP is in a private range (RFC 1918/RFC 4193).
func IsPrivate(ip string) bool {
	parsed := net.ParseIP(ip)
	if parsed == nil {
		return false
	}

	privateRanges := []string{
		"10.0.0.0/8",
		"172.16.0.0/12",
		"192.168.0.0/16",
		"fc00::/7",
	}

	for _, cidr := range privateRanges {
		_, network, _ := net.ParseCIDR(cidr)
		if network.Contains(parsed) {
			return true
		}
	}
	return false
}

// IsLoopback checks if an IP is a loopback address.
func IsLoopback(ip string) bool {
	parsed := net.ParseIP(ip)
	if parsed == nil {
		return false
	}
	return parsed.IsLoopback()
}

// Normalize normalizes an IP address string.
func Normalize(ip string) string {
	// Strip port if present
	if strings.Contains(ip, ":") && !strings.Contains(ip, "::") {
		host, _, err := net.SplitHostPort(ip)
		if err == nil {
			ip = host
		}
	}
	parsed := net.ParseIP(ip)
	if parsed == nil {
		return ip
	}
	return parsed.String()
}
