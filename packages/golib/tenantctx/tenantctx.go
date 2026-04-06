// Package tenantctx provides multi-tenant context isolation middleware.
// Extracts tenant ID from requests, validates it, and propagates it
// through context for downstream use.
package tenantctx

import (
	"context"
	"fmt"
	"net/http"
	"strings"
)

type contextKey struct{}

// FromContext extracts the tenant ID from context.
func FromContext(ctx context.Context) (string, bool) {
	id, ok := ctx.Value(contextKey{}).(string)
	return id, ok
}

// MustFromContext extracts tenant ID or panics.
func MustFromContext(ctx context.Context) string {
	id, ok := FromContext(ctx)
	if !ok {
		panic("tenantctx: tenant ID not found in context")
	}
	return id
}

// WithTenant stores tenant ID in context.
func WithTenant(ctx context.Context, tenantID string) context.Context {
	return context.WithValue(ctx, contextKey{}, tenantID)
}

// Config controls tenant extraction behavior.
type Config struct {
	// HeaderName is the header containing the tenant ID.
	HeaderName string
	// QueryParam is the query parameter containing the tenant ID (fallback).
	QueryParam string
	// Required makes tenant ID mandatory (403 if missing).
	Required bool
	// Validator is an optional function to validate tenant IDs.
	Validator func(tenantID string) error
	// AllowedTenants restricts to specific tenant IDs. Empty = allow all.
	AllowedTenants map[string]bool
}

// DefaultConfig returns sensible defaults.
func DefaultConfig() Config {
	return Config{
		HeaderName: "X-Tenant-ID",
		Required:   true,
	}
}

// Middleware returns HTTP middleware that extracts and validates tenant ID.
func Middleware(cfg Config) func(http.Handler) http.Handler {
	if cfg.HeaderName == "" {
		cfg.HeaderName = "X-Tenant-ID"
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tenantID := r.Header.Get(cfg.HeaderName)

			// Fallback to query parameter.
			if tenantID == "" && cfg.QueryParam != "" {
				tenantID = r.URL.Query().Get(cfg.QueryParam)
			}

			tenantID = strings.TrimSpace(tenantID)

			if tenantID == "" {
				if cfg.Required {
					w.Header().Set("Content-Type", "application/problem+json")
					w.WriteHeader(http.StatusForbidden)
					fmt.Fprintf(w, `{"type":"about:blank","title":"Tenant Required","status":403,"detail":"Tenant ID is required"}`)
					return
				}
				next.ServeHTTP(w, r)
				return
			}

			// Validate against allowlist.
			if len(cfg.AllowedTenants) > 0 && !cfg.AllowedTenants[tenantID] {
				w.Header().Set("Content-Type", "application/problem+json")
				w.WriteHeader(http.StatusForbidden)
				fmt.Fprintf(w, `{"type":"about:blank","title":"Invalid Tenant","status":403,"detail":"Tenant ID is not recognized"}`)
				return
			}

			// Custom validation.
			if cfg.Validator != nil {
				if err := cfg.Validator(tenantID); err != nil {
					w.Header().Set("Content-Type", "application/problem+json")
					w.WriteHeader(http.StatusForbidden)
					fmt.Fprintf(w, `{"type":"about:blank","title":"Invalid Tenant","status":403,"detail":"%s"}`, err.Error())
					return
				}
			}

			ctx := WithTenant(r.Context(), tenantID)

			// Echo tenant ID in response.
			w.Header().Set(cfg.HeaderName, tenantID)

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
