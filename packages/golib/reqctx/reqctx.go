// Package reqctx provides typed request context accessors for
// common HTTP request metadata. Provides a single point of truth
// for extracting user ID, tenant ID, request ID, etc. from context.
package reqctx

import (
	"context"
	"net/http"
)

type contextKey int

const (
	keyUserID contextKey = iota
	keyTenantID
	keyRequestID
	keyCorrelationID
	keySessionID
	keyRoles
	keyClientIP
)

// SetUserID stores the user ID in context.
func SetUserID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, keyUserID, id)
}

// UserID extracts the user ID from context.
func UserID(ctx context.Context) string {
	v, _ := ctx.Value(keyUserID).(string)
	return v
}

// SetTenantID stores the tenant ID in context.
func SetTenantID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, keyTenantID, id)
}

// TenantID extracts the tenant ID from context.
func TenantID(ctx context.Context) string {
	v, _ := ctx.Value(keyTenantID).(string)
	return v
}

// SetRequestID stores the request ID in context.
func SetRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, keyRequestID, id)
}

// RequestID extracts the request ID from context.
func RequestID(ctx context.Context) string {
	v, _ := ctx.Value(keyRequestID).(string)
	return v
}

// SetCorrelationID stores the correlation ID in context.
func SetCorrelationID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, keyCorrelationID, id)
}

// CorrelationID extracts the correlation ID from context.
func CorrelationID(ctx context.Context) string {
	v, _ := ctx.Value(keyCorrelationID).(string)
	return v
}

// SetSessionID stores the session ID in context.
func SetSessionID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, keySessionID, id)
}

// SessionID extracts the session ID from context.
func SessionID(ctx context.Context) string {
	v, _ := ctx.Value(keySessionID).(string)
	return v
}

// SetRoles stores user roles in context.
func SetRoles(ctx context.Context, roles []string) context.Context {
	return context.WithValue(ctx, keyRoles, roles)
}

// Roles extracts user roles from context.
func Roles(ctx context.Context) []string {
	v, _ := ctx.Value(keyRoles).([]string)
	return v
}

// HasRole checks if the context has a specific role.
func HasRole(ctx context.Context, role string) bool {
	for _, r := range Roles(ctx) {
		if r == role {
			return true
		}
	}
	return false
}

// SetClientIP stores the client IP in context.
func SetClientIP(ctx context.Context, ip string) context.Context {
	return context.WithValue(ctx, keyClientIP, ip)
}

// ClientIP extracts the client IP from context.
func ClientIP(ctx context.Context) string {
	v, _ := ctx.Value(keyClientIP).(string)
	return v
}

// EnrichFromRequest extracts common values from request headers
// and stores them in context.
func EnrichFromRequest(r *http.Request) *http.Request {
	ctx := r.Context()

	if v := r.Header.Get("X-User-ID"); v != "" {
		ctx = SetUserID(ctx, v)
	}
	if v := r.Header.Get("X-Tenant-ID"); v != "" {
		ctx = SetTenantID(ctx, v)
	}
	if v := r.Header.Get("X-Request-ID"); v != "" {
		ctx = SetRequestID(ctx, v)
	}
	if v := r.Header.Get("X-Correlation-ID"); v != "" {
		ctx = SetCorrelationID(ctx, v)
	}

	return r.WithContext(ctx)
}

// Middleware returns HTTP middleware that enriches the request context.
func Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(w, EnrichFromRequest(r))
	})
}
