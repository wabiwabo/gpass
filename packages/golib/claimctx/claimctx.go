// Package claimctx provides typed JWT claim extraction from context.
// Stores parsed token claims for downstream handlers to access
// user identity, roles, scopes, and token metadata.
package claimctx

import (
	"context"
	"time"
)

type contextKey int

const keyClaims contextKey = iota

// Claims represents parsed JWT claims.
type Claims struct {
	Subject   string            `json:"sub"`
	Issuer    string            `json:"iss"`
	Audience  []string          `json:"aud"`
	ExpiresAt time.Time         `json:"exp"`
	IssuedAt  time.Time         `json:"iat"`
	NotBefore time.Time         `json:"nbf,omitempty"`
	JWTID     string            `json:"jti,omitempty"`
	Scopes    []string          `json:"scopes,omitempty"`
	Roles     []string          `json:"roles,omitempty"`
	TenantID  string            `json:"tenant_id,omitempty"`
	ClientID  string            `json:"client_id,omitempty"`
	TokenType string            `json:"token_type,omitempty"` // "access", "refresh", "id"
	Extra     map[string]string `json:"extra,omitempty"`
}

// IsExpired checks if the token has expired.
func (c Claims) IsExpired() bool {
	return !c.ExpiresAt.IsZero() && time.Now().After(c.ExpiresAt)
}

// HasScope checks if a scope is present.
func (c Claims) HasScope(scope string) bool {
	for _, s := range c.Scopes {
		if s == scope {
			return true
		}
	}
	return false
}

// HasRole checks if a role is present.
func (c Claims) HasRole(role string) bool {
	for _, r := range c.Roles {
		if r == role {
			return true
		}
	}
	return false
}

// HasAudience checks if an audience is present.
func (c Claims) HasAudience(aud string) bool {
	for _, a := range c.Audience {
		if a == aud {
			return true
		}
	}
	return false
}

// Set stores claims in context.
func Set(ctx context.Context, claims Claims) context.Context {
	return context.WithValue(ctx, keyClaims, claims)
}

// Get extracts claims from context. Returns zero Claims if not set.
func Get(ctx context.Context) Claims {
	v, _ := ctx.Value(keyClaims).(Claims)
	return v
}

// Subject returns just the subject from context claims.
func Subject(ctx context.Context) string {
	return Get(ctx).Subject
}

// TenantID returns just the tenant ID from context claims.
func TenantID(ctx context.Context) string {
	return Get(ctx).TenantID
}

// HasScope checks if context claims have the given scope.
func HasScope(ctx context.Context, scope string) bool {
	return Get(ctx).HasScope(scope)
}

// HasRole checks if context claims have the given role.
func HasRole(ctx context.Context, role string) bool {
	return Get(ctx).HasRole(role)
}
