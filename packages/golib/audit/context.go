package audit

import (
	"context"
	"net"
	"net/http"
	"strings"
	"time"
)

// Context holds audit information extracted from an HTTP request.
type Context struct {
	RequestID   string
	UserID      string
	UserAgent   string
	IPAddress   string
	ServiceName string
	Timestamp   time.Time
}

// FromRequest extracts audit context from an HTTP request.
// Gets RequestID from X-Request-Id header, UserID from X-User-ID,
// IP from X-Forwarded-For (first entry) or RemoteAddr.
func FromRequest(r *http.Request, serviceName string) Context {
	return Context{
		RequestID:   r.Header.Get("X-Request-Id"),
		UserID:      r.Header.Get("X-User-ID"),
		UserAgent:   r.UserAgent(),
		IPAddress:   RealIP(r),
		ServiceName: serviceName,
		Timestamp:   time.Now(),
	}
}

// ctxKey is the key type for context values.
type ctxKey struct{}

// WithContext stores audit context in the request context.
func WithContext(ctx context.Context, ac Context) context.Context {
	return context.WithValue(ctx, ctxKey{}, ac)
}

// GetContext retrieves audit context from context.
func GetContext(ctx context.Context) (Context, bool) {
	ac, ok := ctx.Value(ctxKey{}).(Context)
	return ac, ok
}

// RealIP extracts the client's real IP from X-Forwarded-For or
// X-Real-IP headers, falling back to RemoteAddr.
// Returns only the first IP in X-Forwarded-For chain (client IP).
func RealIP(r *http.Request) string {
	// Check X-Forwarded-For first.
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		// Take the first IP in the comma-separated list.
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

	// Fall back to RemoteAddr, stripping the port.
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		// RemoteAddr might not have a port.
		return r.RemoteAddr
	}
	return ip
}
