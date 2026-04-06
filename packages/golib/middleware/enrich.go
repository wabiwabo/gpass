package middleware

import (
	"context"
	"net"
	"net/http"
	"strings"
)

type enrichKey struct{}

// ClientInfo holds enriched client information extracted from request headers.
type ClientInfo struct {
	// IP is the client's IP address (from X-Forwarded-For or RemoteAddr).
	IP string
	// UserAgent is the User-Agent header.
	UserAgent string
	// Country is from CF-IPCountry or X-Country header (set by CDN/gateway).
	Country string
	// DeviceType is derived from User-Agent: "mobile", "tablet", "desktop", "bot", "unknown".
	DeviceType string
	// AcceptLanguage is the client's preferred language.
	AcceptLanguage string
	// Referer is the referring URL.
	Referer string
	// ContentType of the request.
	ContentType string
}

// Enrich extracts client information from request headers and stores in context.
func Enrich(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		info := ClientInfo{
			IP:             extractIP(r),
			UserAgent:      r.Header.Get("User-Agent"),
			Country:        extractCountry(r),
			DeviceType:     detectDevice(r.Header.Get("User-Agent")),
			AcceptLanguage: r.Header.Get("Accept-Language"),
			Referer:        r.Header.Get("Referer"),
			ContentType:    r.Header.Get("Content-Type"),
		}

		ctx := context.WithValue(r.Context(), enrichKey{}, info)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// ClientInfoFromContext retrieves ClientInfo from the request context.
func ClientInfoFromContext(ctx context.Context) (ClientInfo, bool) {
	info, ok := ctx.Value(enrichKey{}).(ClientInfo)
	return info, ok
}

func extractIP(r *http.Request) string {
	// Try X-Forwarded-For first (may contain chain: "client, proxy1, proxy2").
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.SplitN(xff, ",", 2)
		ip := strings.TrimSpace(parts[0])
		if ip != "" {
			return ip
		}
	}
	// Try X-Real-IP (set by some reverse proxies).
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

func extractCountry(r *http.Request) string {
	// Cloudflare sets CF-IPCountry.
	if cc := r.Header.Get("CF-IPCountry"); cc != "" {
		return strings.ToUpper(cc)
	}
	// Some gateways set X-Country.
	if cc := r.Header.Get("X-Country"); cc != "" {
		return strings.ToUpper(cc)
	}
	return ""
}

func detectDevice(ua string) string {
	if ua == "" {
		return "unknown"
	}
	lower := strings.ToLower(ua)

	// Bot detection.
	botKeywords := []string{"bot", "crawler", "spider", "scraper", "curl", "wget", "httpie"}
	for _, kw := range botKeywords {
		if strings.Contains(lower, kw) {
			return "bot"
		}
	}

	// Mobile detection.
	mobileKeywords := []string{"iphone", "android", "mobile", "blackberry", "windows phone"}
	for _, kw := range mobileKeywords {
		if strings.Contains(lower, kw) {
			// Distinguish tablet from phone.
			if strings.Contains(lower, "ipad") || (strings.Contains(lower, "android") && !strings.Contains(lower, "mobile")) {
				return "tablet"
			}
			return "mobile"
		}
	}

	if strings.Contains(lower, "ipad") {
		return "tablet"
	}

	return "desktop"
}
