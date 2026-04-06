// Package consentctx provides typed consent context accessors
// for managing user data sharing consent per UU PDP No. 27/2022.
// Stores granted consent scopes in request context for downstream
// authorization checks.
package consentctx

import (
	"context"
	"sort"
	"time"
)

type contextKey int

const keyConsent contextKey = iota

// Consent represents a user's data sharing consent.
type Consent struct {
	UserID    string    `json:"user_id"`
	Scopes    []string  `json:"scopes"`
	GrantedAt time.Time `json:"granted_at"`
	ExpiresAt time.Time `json:"expires_at,omitempty"`
	Purpose   string    `json:"purpose"`
	Processor string    `json:"processor"` // entity processing the data
}

// IsExpired checks if the consent has expired.
func (c Consent) IsExpired() bool {
	if c.ExpiresAt.IsZero() {
		return false
	}
	return time.Now().After(c.ExpiresAt)
}

// HasScope checks if a specific scope is consented.
func (c Consent) HasScope(scope string) bool {
	for _, s := range c.Scopes {
		if s == scope {
			return true
		}
	}
	return false
}

// HasAllScopes checks if all required scopes are consented.
func (c Consent) HasAllScopes(required ...string) bool {
	set := make(map[string]bool, len(c.Scopes))
	for _, s := range c.Scopes {
		set[s] = true
	}
	for _, r := range required {
		if !set[r] {
			return false
		}
	}
	return true
}

// Set stores consent in context.
func Set(ctx context.Context, consent Consent) context.Context {
	return context.WithValue(ctx, keyConsent, consent)
}

// Get extracts consent from context.
func Get(ctx context.Context) (Consent, bool) {
	v, ok := ctx.Value(keyConsent).(Consent)
	return v, ok
}

// MustGet extracts consent, returning zero value if not set.
func MustGet(ctx context.Context) Consent {
	v, _ := ctx.Value(keyConsent).(Consent)
	return v
}

// HasScope checks if context consent has the given scope.
func HasScope(ctx context.Context, scope string) bool {
	c, ok := Get(ctx)
	if !ok {
		return false
	}
	if c.IsExpired() {
		return false
	}
	return c.HasScope(scope)
}

// HasAllScopes checks if context consent has all required scopes.
func HasAllScopes(ctx context.Context, required ...string) bool {
	c, ok := Get(ctx)
	if !ok {
		return false
	}
	if c.IsExpired() {
		return false
	}
	return c.HasAllScopes(required...)
}

// SortedScopes returns the consent scopes sorted alphabetically.
func SortedScopes(ctx context.Context) []string {
	c, ok := Get(ctx)
	if !ok || len(c.Scopes) == 0 {
		return nil
	}
	result := make([]string, len(c.Scopes))
	copy(result, c.Scopes)
	sort.Strings(result)
	return result
}

// Common consent scopes for GarudaPass PDP compliance.
const (
	ScopeNIK       = "nik"
	ScopeName      = "name"
	ScopeEmail     = "email"
	ScopePhone     = "phone"
	ScopeAddress   = "address"
	ScopeBirthDate = "birth_date"
	ScopePhoto     = "photo"
	ScopeReligion  = "religion"
	ScopeMarital   = "marital_status"
	ScopeBloodType = "blood_type"
	ScopeFamily    = "family"
)
