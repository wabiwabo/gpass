// Package mwtenant provides multi-tenant isolation middleware.
// Extracts tenant ID from headers or path, validates it, and
// injects into request context for downstream handlers.
package mwtenant

import (
	"context"
	"net/http"
	"regexp"
	"strings"
)

type contextKey int

const keyTenantID contextKey = iota

// SetTenantID stores the tenant ID in context.
func SetTenantID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, keyTenantID, id)
}

// TenantID extracts the tenant ID from context.
func TenantID(ctx context.Context) string {
	v, _ := ctx.Value(keyTenantID).(string)
	return v
}

// Source defines where to extract the tenant ID from.
type Source string

const (
	SourceHeader Source = "header"
	SourcePath   Source = "path"
)

// Config controls tenant extraction behavior.
type Config struct {
	Source     Source
	HeaderName string // header name (default "X-Tenant-ID")
	PathIndex  int    // path segment index for path source
	Required   bool   // reject if tenant not found
	Validator  func(string) bool // optional tenant validator
}

var tenantPattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_-]{1,62}[a-zA-Z0-9]$`)

// ValidTenantID checks if a tenant ID has valid format.
func ValidTenantID(id string) bool {
	return tenantPattern.MatchString(id)
}

// Middleware returns tenant extraction middleware.
func Middleware(cfg Config) func(http.Handler) http.Handler {
	if cfg.HeaderName == "" {
		cfg.HeaderName = "X-Tenant-ID"
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var tenantID string

			switch cfg.Source {
			case SourcePath:
				segments := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
				if cfg.PathIndex >= 0 && cfg.PathIndex < len(segments) {
					tenantID = segments[cfg.PathIndex]
				}
			default: // header
				tenantID = r.Header.Get(cfg.HeaderName)
			}

			if tenantID == "" && cfg.Required {
				http.Error(w, `{"error":"tenant_required","message":"tenant ID is required"}`, http.StatusBadRequest)
				return
			}

			if tenantID != "" {
				if !ValidTenantID(tenantID) {
					http.Error(w, `{"error":"invalid_tenant","message":"invalid tenant ID format"}`, http.StatusBadRequest)
					return
				}
				if cfg.Validator != nil && !cfg.Validator(tenantID) {
					http.Error(w, `{"error":"unknown_tenant","message":"tenant not found"}`, http.StatusNotFound)
					return
				}
			}

			if tenantID != "" {
				ctx := SetTenantID(r.Context(), tenantID)
				r = r.WithContext(ctx)
			}

			next.ServeHTTP(w, r)
		})
	}
}
