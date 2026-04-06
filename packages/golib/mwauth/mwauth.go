// Package mwauth provides authentication middleware that extracts
// and validates Bearer tokens from the Authorization header. Supports
// pluggable token validators for different auth strategies.
package mwauth

import (
	"context"
	"net/http"
	"strings"
)

type contextKey int

const keyToken contextKey = iota

// TokenInfo holds validated token information.
type TokenInfo struct {
	Raw       string
	Subject   string
	TokenType string
}

// Validator validates a bearer token and returns token info.
type Validator func(ctx context.Context, token string) (TokenInfo, error)

// SetToken stores token info in context.
func SetToken(ctx context.Context, info TokenInfo) context.Context {
	return context.WithValue(ctx, keyToken, info)
}

// GetToken extracts token info from context.
func GetToken(ctx context.Context) (TokenInfo, bool) {
	v, ok := ctx.Value(keyToken).(TokenInfo)
	return v, ok
}

// Config controls auth middleware behavior.
type Config struct {
	Validator    Validator
	SkipPaths    []string
	ErrorMessage string
}

// Middleware returns Bearer token authentication middleware.
func Middleware(cfg Config) func(http.Handler) http.Handler {
	skipSet := make(map[string]bool, len(cfg.SkipPaths))
	for _, p := range cfg.SkipPaths {
		skipSet[p] = true
	}

	errMsg := cfg.ErrorMessage
	if errMsg == "" {
		errMsg = `{"error":"unauthorized","message":"valid bearer token required"}`
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if skipSet[r.URL.Path] {
				next.ServeHTTP(w, r)
				return
			}

			auth := r.Header.Get("Authorization")
			if auth == "" || !strings.HasPrefix(auth, "Bearer ") {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnauthorized)
				w.Write([]byte(errMsg))
				return
			}

			token := strings.TrimPrefix(auth, "Bearer ")
			if token == "" {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnauthorized)
				w.Write([]byte(errMsg))
				return
			}

			info, err := cfg.Validator(r.Context(), token)
			if err != nil {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnauthorized)
				w.Write([]byte(errMsg))
				return
			}

			info.Raw = token
			ctx := SetToken(r.Context(), info)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// ExtractBearer extracts the bearer token from an Authorization header value.
func ExtractBearer(authHeader string) string {
	if !strings.HasPrefix(authHeader, "Bearer ") {
		return ""
	}
	return strings.TrimPrefix(authHeader, "Bearer ")
}
