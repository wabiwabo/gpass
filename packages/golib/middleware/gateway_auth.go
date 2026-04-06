package middleware

import (
	"context"
	"net/http"
	"strings"
)

// AuthMethod represents an authentication strategy.
type AuthMethod int

const (
	AuthNone    AuthMethod = iota
	AuthAPIKey             // X-API-Key header
	AuthBearer             // Authorization: Bearer <token>
	AuthSession            // Session cookie via BFF
	AuthService            // X-Service-Signature (internal)
)

// String returns the human-readable name of the auth method.
func (m AuthMethod) String() string {
	switch m {
	case AuthAPIKey:
		return "api_key"
	case AuthBearer:
		return "bearer"
	case AuthSession:
		return "session"
	case AuthService:
		return "service"
	default:
		return "none"
	}
}

// AuthResult contains the result of authentication.
type AuthResult struct {
	Authenticated bool
	Method        AuthMethod
	Subject       string            // user ID or app ID
	Metadata      map[string]string // additional context
}

type authResultKeyType struct{}

var authResultKey = authResultKeyType{}

// Authenticator validates credentials using multiple methods.
type Authenticator struct {
	apiKeyValidator  func(key string) (*AuthResult, error)
	tokenValidator   func(token string) (*AuthResult, error)
	sessionValidator func(cookie string) (*AuthResult, error)
	serviceValidator func(sig string, r *http.Request) (*AuthResult, error)
}

// AuthOption configures an Authenticator.
type AuthOption func(*Authenticator)

// WithAPIKeyValidator sets the API key validation function.
func WithAPIKeyValidator(fn func(string) (*AuthResult, error)) AuthOption {
	return func(a *Authenticator) {
		a.apiKeyValidator = fn
	}
}

// WithTokenValidator sets the bearer token validation function.
func WithTokenValidator(fn func(string) (*AuthResult, error)) AuthOption {
	return func(a *Authenticator) {
		a.tokenValidator = fn
	}
}

// WithSessionValidator sets the session cookie validation function.
func WithSessionValidator(fn func(string) (*AuthResult, error)) AuthOption {
	return func(a *Authenticator) {
		a.sessionValidator = fn
	}
}

// WithServiceValidator sets the service signature validation function.
func WithServiceValidator(fn func(string, *http.Request) (*AuthResult, error)) AuthOption {
	return func(a *Authenticator) {
		a.serviceValidator = fn
	}
}

// NewAuthenticator creates a new Authenticator with the given options.
func NewAuthenticator(opts ...AuthOption) *Authenticator {
	a := &Authenticator{}
	for _, opt := range opts {
		opt(a)
	}
	return a
}

// Authenticate returns middleware that authenticates using configured methods.
// Tries methods in order: Service > Bearer > APIKey > Session.
// Sets X-Auth-Method and X-Auth-Subject headers for downstream services.
func (a *Authenticator) Authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var result *AuthResult

		// 1. Service signature (highest priority).
		if a.serviceValidator != nil {
			if sig := r.Header.Get("X-Service-Signature"); sig != "" {
				res, err := a.serviceValidator(sig, r)
				if err == nil && res != nil && res.Authenticated {
					result = res
					result.Method = AuthService
				}
			}
		}

		// 2. Bearer token.
		if result == nil && a.tokenValidator != nil {
			if auth := r.Header.Get("Authorization"); strings.HasPrefix(auth, "Bearer ") {
				token := strings.TrimPrefix(auth, "Bearer ")
				res, err := a.tokenValidator(token)
				if err == nil && res != nil && res.Authenticated {
					result = res
					result.Method = AuthBearer
				}
			}
		}

		// 3. API key.
		if result == nil && a.apiKeyValidator != nil {
			if key := r.Header.Get("X-API-Key"); key != "" {
				res, err := a.apiKeyValidator(key)
				if err == nil && res != nil && res.Authenticated {
					result = res
					result.Method = AuthAPIKey
				}
			}
		}

		// 4. Session cookie.
		if result == nil && a.sessionValidator != nil {
			if cookie, err := r.Cookie("session"); err == nil && cookie.Value != "" {
				res, err := a.sessionValidator(cookie.Value)
				if err == nil && res != nil && res.Authenticated {
					result = res
					result.Method = AuthSession
				}
			}
		}

		// No valid credentials found.
		if result == nil {
			http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
			return
		}

		// Set downstream headers.
		w.Header().Set("X-Auth-Method", result.Method.String())
		w.Header().Set("X-Auth-Subject", result.Subject)

		// Store result in context.
		ctx := context.WithValue(r.Context(), authResultKey, result)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// GetAuthResult retrieves auth result from request context.
func GetAuthResult(ctx context.Context) (*AuthResult, bool) {
	if ctx == nil {
		return nil, false
	}
	result, ok := ctx.Value(authResultKey).(*AuthResult)
	return result, ok
}
