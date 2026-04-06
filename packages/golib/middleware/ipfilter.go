package middleware

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strings"
)

// IPFilter returns middleware that filters requests by IP address.
// mode: "allow" (only listed IPs allowed) or "deny" (listed IPs blocked).
// cidrs is a list of CIDR notation strings (e.g., "10.0.0.0/8", "192.168.1.1/32").
func IPFilter(mode string, cidrs []string) (func(http.Handler) http.Handler, error) {
	if mode != "allow" && mode != "deny" {
		return nil, fmt.Errorf("ipfilter: invalid mode %q, must be \"allow\" or \"deny\"", mode)
	}

	networks := make([]*net.IPNet, 0, len(cidrs))
	for _, cidr := range cidrs {
		_, network, err := net.ParseCIDR(cidr)
		if err != nil {
			return nil, fmt.Errorf("ipfilter: invalid CIDR %q: %w", cidr, err)
		}
		networks = append(networks, network)
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := extractClientIP(r)
			parsed := net.ParseIP(ip)
			if parsed == nil {
				writeIPFilterError(w, http.StatusForbidden, "invalid client IP")
				return
			}

			matched := matchesAny(parsed, networks)

			switch mode {
			case "allow":
				if !matched {
					writeIPFilterError(w, http.StatusForbidden, "IP address not allowed")
					return
				}
			case "deny":
				if matched {
					writeIPFilterError(w, http.StatusForbidden, "IP address denied")
					return
				}
			}

			next.ServeHTTP(w, r)
		})
	}, nil
}

// IPAllowlist is a convenience wrapper for IPFilter with mode "allow".
func IPAllowlist(cidrs []string) (func(http.Handler) http.Handler, error) {
	return IPFilter("allow", cidrs)
}

// IPDenylist is a convenience wrapper for IPFilter with mode "deny".
func IPDenylist(cidrs []string) (func(http.Handler) http.Handler, error) {
	return IPFilter("deny", cidrs)
}

// extractClientIP extracts the client IP from the request, checking
// X-Forwarded-For, X-Real-IP, and finally RemoteAddr.
func extractClientIP(r *http.Request) string {
	// Check X-Forwarded-For first (may contain multiple IPs, use the first).
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.SplitN(xff, ",", 2)
		ip := strings.TrimSpace(parts[0])
		if ip != "" {
			return ip
		}
	}

	// Check X-Real-IP.
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return strings.TrimSpace(xri)
	}

	// Fall back to RemoteAddr.
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// matchesAny checks if the IP matches any of the networks.
func matchesAny(ip net.IP, networks []*net.IPNet) bool {
	for _, network := range networks {
		if network.Contains(ip) {
			return true
		}
	}
	return false
}

func writeIPFilterError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"error":   http.StatusText(status),
		"message": message,
		"code":    status,
	})
}
