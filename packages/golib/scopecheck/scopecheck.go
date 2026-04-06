// Package scopecheck provides OAuth2 scope validation middleware.
// Verifies that the request context contains required scopes,
// returning 403 Forbidden for insufficient permissions.
package scopecheck

import (
	"context"
	"encoding/json"
	"net/http"
	"sort"
	"strings"
)

type contextKey int

const keyScopeSet contextKey = iota

// SetScopes stores the granted scopes in context.
func SetScopes(ctx context.Context, scopes []string) context.Context {
	set := make(map[string]bool, len(scopes))
	for _, s := range scopes {
		s = strings.TrimSpace(s)
		if s != "" {
			set[s] = true
		}
	}
	return context.WithValue(ctx, keyScopeSet, set)
}

// GetScopes returns the granted scopes from context.
func GetScopes(ctx context.Context) []string {
	set, _ := ctx.Value(keyScopeSet).(map[string]bool)
	if len(set) == 0 {
		return nil
	}
	result := make([]string, 0, len(set))
	for s := range set {
		result = append(result, s)
	}
	sort.Strings(result)
	return result
}

// HasScope checks if a specific scope is granted.
func HasScope(ctx context.Context, scope string) bool {
	set, _ := ctx.Value(keyScopeSet).(map[string]bool)
	return set[scope]
}

// HasAllScopes checks if all required scopes are granted.
func HasAllScopes(ctx context.Context, required ...string) bool {
	set, _ := ctx.Value(keyScopeSet).(map[string]bool)
	for _, s := range required {
		if !set[s] {
			return false
		}
	}
	return true
}

// HasAnyScope checks if at least one of the scopes is granted.
func HasAnyScope(ctx context.Context, scopes ...string) bool {
	set, _ := ctx.Value(keyScopeSet).(map[string]bool)
	for _, s := range scopes {
		if set[s] {
			return true
		}
	}
	return false
}

// Require returns middleware that requires all specified scopes.
func Require(scopes ...string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !HasAllScopes(r.Context(), scopes...) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusForbidden)
				json.NewEncoder(w).Encode(map[string]interface{}{
					"error":           "insufficient_scope",
					"required_scopes": scopes,
					"description":     "The request requires higher privileges than provided by the access token.",
				})
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// RequireAny returns middleware that requires at least one of the scopes.
func RequireAny(scopes ...string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !HasAnyScope(r.Context(), scopes...) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusForbidden)
				json.NewEncoder(w).Encode(map[string]interface{}{
					"error":           "insufficient_scope",
					"required_scopes": scopes,
					"description":     "The request requires at least one of the specified scopes.",
				})
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// ParseSpaceDelimited parses OAuth2 space-delimited scope string.
// e.g., "openid profile email" → ["openid", "profile", "email"]
func ParseSpaceDelimited(s string) []string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	parts := strings.Fields(s)
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}

// ScopeMiddleware extracts scopes from a header and stores in context.
func ScopeMiddleware(headerName string) func(http.Handler) http.Handler {
	if headerName == "" {
		headerName = "X-Scopes"
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			scopeStr := r.Header.Get(headerName)
			if scopeStr != "" {
				scopes := ParseSpaceDelimited(scopeStr)
				ctx := SetScopes(r.Context(), scopes)
				r = r.WithContext(ctx)
			}
			next.ServeHTTP(w, r)
		})
	}
}
