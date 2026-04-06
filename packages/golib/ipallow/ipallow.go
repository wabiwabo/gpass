// Package ipallow provides IP-based access control with allowlists
// and blocklists. Supports CIDR ranges, individual IPs, and
// middleware for HTTP request filtering.
package ipallow

import (
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"
)

// List manages IP allow/block lists.
type List struct {
	mu       sync.RWMutex
	allowed  []net.IPNet
	blocked  []net.IPNet
	mode     Mode
}

// Mode determines the access control behavior.
type Mode int

const (
	ModeAllowList Mode = iota // Only listed IPs are allowed.
	ModeBlockList             // Listed IPs are blocked, all others allowed.
)

// New creates an IP access control list.
func New(mode Mode) *List {
	return &List{mode: mode}
}

// Allow adds an IP or CIDR to the allowlist.
func (l *List) Allow(cidr string) error {
	_, ipNet, err := parseCIDR(cidr)
	if err != nil {
		return err
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	l.allowed = append(l.allowed, *ipNet)
	return nil
}

// Block adds an IP or CIDR to the blocklist.
func (l *List) Block(cidr string) error {
	_, ipNet, err := parseCIDR(cidr)
	if err != nil {
		return err
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	l.blocked = append(l.blocked, *ipNet)
	return nil
}

// IsAllowed checks if an IP is allowed.
func (l *List) IsAllowed(ip string) bool {
	parsed := net.ParseIP(ip)
	if parsed == nil {
		return false
	}

	l.mu.RLock()
	defer l.mu.RUnlock()

	// Always check blocklist first.
	for _, blocked := range l.blocked {
		if blocked.Contains(parsed) {
			return false
		}
	}

	switch l.mode {
	case ModeAllowList:
		for _, allowed := range l.allowed {
			if allowed.Contains(parsed) {
				return true
			}
		}
		return len(l.allowed) == 0 // Empty allowlist = allow all.
	case ModeBlockList:
		return true // Not in blocklist = allowed.
	}

	return false
}

// Middleware returns HTTP middleware that enforces the IP list.
func (l *List) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := extractIP(r)
		if !l.IsAllowed(ip) {
			w.Header().Set("Content-Type", "application/problem+json")
			w.WriteHeader(http.StatusForbidden)
			fmt.Fprintf(w, `{"type":"about:blank","title":"Forbidden","status":403,"detail":"IP address not allowed"}`)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// Count returns the number of entries.
func (l *List) Count() (allowed, blocked int) {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return len(l.allowed), len(l.blocked)
}

func extractIP(r *http.Request) string {
	// Check X-Forwarded-For first (from reverse proxy).
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.SplitN(xff, ",", 2)
		ip := strings.TrimSpace(parts[0])
		if net.ParseIP(ip) != nil {
			return ip
		}
	}
	// Check X-Real-IP.
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		if net.ParseIP(xri) != nil {
			return xri
		}
	}
	// Fall back to RemoteAddr.
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func parseCIDR(s string) (net.IP, *net.IPNet, error) {
	// If it's a plain IP, convert to /32 or /128.
	if !strings.Contains(s, "/") {
		ip := net.ParseIP(s)
		if ip == nil {
			return nil, nil, fmt.Errorf("ipallow: invalid IP %q", s)
		}
		if ip.To4() != nil {
			s = s + "/32"
		} else {
			s = s + "/128"
		}
	}
	return net.ParseCIDR(s)
}
