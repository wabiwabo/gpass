package claimctx

import (
	"context"
	"testing"
	"time"
)

func TestClaims_IsExpired(t *testing.T) {
	tests := []struct {
		name    string
		exp     time.Time
		expired bool
	}{
		{"not expired", time.Now().Add(1 * time.Hour), false},
		{"expired", time.Now().Add(-1 * time.Hour), true},
		{"zero time", time.Time{}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := Claims{ExpiresAt: tt.exp}
			if c.IsExpired() != tt.expired {
				t.Errorf("IsExpired = %v, want %v", c.IsExpired(), tt.expired)
			}
		})
	}
}

func TestClaims_HasScope(t *testing.T) {
	c := Claims{Scopes: []string{"openid", "profile", "email"}}

	if !c.HasScope("openid") {
		t.Error("should have openid")
	}
	if c.HasScope("admin") {
		t.Error("should not have admin")
	}
	if c.HasScope("") {
		t.Error("should not have empty scope")
	}
}

func TestClaims_HasRole(t *testing.T) {
	c := Claims{Roles: []string{"admin", "operator"}}

	if !c.HasRole("admin") {
		t.Error("should have admin")
	}
	if c.HasRole("user") {
		t.Error("should not have user")
	}
}

func TestClaims_HasAudience(t *testing.T) {
	c := Claims{Audience: []string{"api.garudapass.id", "web.garudapass.id"}}

	if !c.HasAudience("api.garudapass.id") {
		t.Error("should have api audience")
	}
	if c.HasAudience("other.com") {
		t.Error("should not have other audience")
	}
}

func TestSetGet(t *testing.T) {
	claims := Claims{
		Subject:   "user-123",
		Issuer:    "https://auth.garudapass.id",
		Audience:  []string{"api.garudapass.id"},
		ExpiresAt: time.Now().Add(1 * time.Hour),
		IssuedAt:  time.Now(),
		Scopes:    []string{"openid", "profile"},
		Roles:     []string{"user"},
		TenantID:  "tenant-abc",
		ClientID:  "client-xyz",
		TokenType: "access",
	}

	ctx := Set(context.Background(), claims)
	got := Get(ctx)

	if got.Subject != "user-123" {
		t.Errorf("Subject = %q", got.Subject)
	}
	if got.Issuer != "https://auth.garudapass.id" {
		t.Errorf("Issuer = %q", got.Issuer)
	}
	if got.TenantID != "tenant-abc" {
		t.Errorf("TenantID = %q", got.TenantID)
	}
	if got.ClientID != "client-xyz" {
		t.Errorf("ClientID = %q", got.ClientID)
	}
	if got.TokenType != "access" {
		t.Errorf("TokenType = %q", got.TokenType)
	}
}

func TestGet_EmptyContext(t *testing.T) {
	got := Get(context.Background())
	if got.Subject != "" {
		t.Errorf("Subject = %q, want empty", got.Subject)
	}
}

func TestSubject(t *testing.T) {
	ctx := Set(context.Background(), Claims{Subject: "user-456"})
	if Subject(ctx) != "user-456" {
		t.Errorf("Subject = %q", Subject(ctx))
	}
}

func TestTenantID(t *testing.T) {
	ctx := Set(context.Background(), Claims{TenantID: "tenant-789"})
	if TenantID(ctx) != "tenant-789" {
		t.Errorf("TenantID = %q", TenantID(ctx))
	}
}

func TestHasScope_Context(t *testing.T) {
	ctx := Set(context.Background(), Claims{Scopes: []string{"read", "write"}})

	if !HasScope(ctx, "read") {
		t.Error("should have read scope")
	}
	if HasScope(ctx, "admin") {
		t.Error("should not have admin scope")
	}
}

func TestHasScope_EmptyContext(t *testing.T) {
	if HasScope(context.Background(), "anything") {
		t.Error("empty context should not have scopes")
	}
}

func TestHasRole_Context(t *testing.T) {
	ctx := Set(context.Background(), Claims{Roles: []string{"admin"}})

	if !HasRole(ctx, "admin") {
		t.Error("should have admin role")
	}
	if HasRole(ctx, "user") {
		t.Error("should not have user role")
	}
}

func TestClaims_Extra(t *testing.T) {
	c := Claims{
		Subject: "user-1",
		Extra:   map[string]string{"custom_field": "custom_value"},
	}
	ctx := Set(context.Background(), c)
	got := Get(ctx)
	if got.Extra["custom_field"] != "custom_value" {
		t.Errorf("Extra = %v", got.Extra)
	}
}

func TestClaims_EmptyScopes(t *testing.T) {
	c := Claims{}
	if c.HasScope("anything") {
		t.Error("empty scopes should not match")
	}
	if c.HasRole("anything") {
		t.Error("empty roles should not match")
	}
	if c.HasAudience("anything") {
		t.Error("empty audience should not match")
	}
}
