package consentctx

import (
	"context"
	"testing"
	"time"
)

func TestConsent_HasScope(t *testing.T) {
	c := Consent{Scopes: []string{ScopeNIK, ScopeEmail, ScopeName}}

	if !c.HasScope(ScopeNIK) {
		t.Error("should have NIK scope")
	}
	if c.HasScope(ScopePhone) {
		t.Error("should not have Phone scope")
	}
}

func TestConsent_HasAllScopes(t *testing.T) {
	c := Consent{Scopes: []string{ScopeNIK, ScopeEmail, ScopeName}}

	if !c.HasAllScopes(ScopeNIK, ScopeEmail) {
		t.Error("should have NIK and email")
	}
	if c.HasAllScopes(ScopeNIK, ScopePhone) {
		t.Error("should not have all when phone missing")
	}
}

func TestConsent_IsExpired(t *testing.T) {
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
			c := Consent{ExpiresAt: tt.exp}
			if c.IsExpired() != tt.expired {
				t.Errorf("IsExpired = %v, want %v", c.IsExpired(), tt.expired)
			}
		})
	}
}

func TestSetGet(t *testing.T) {
	consent := Consent{
		UserID:    "user-123",
		Scopes:    []string{ScopeNIK, ScopeEmail},
		GrantedAt: time.Now(),
		Purpose:   "identity verification",
		Processor: "garudapass",
	}

	ctx := Set(context.Background(), consent)
	got, ok := Get(ctx)
	if !ok {
		t.Fatal("should find consent")
	}
	if got.UserID != "user-123" {
		t.Errorf("UserID = %q", got.UserID)
	}
	if got.Purpose != "identity verification" {
		t.Errorf("Purpose = %q", got.Purpose)
	}
}

func TestGet_EmptyContext(t *testing.T) {
	_, ok := Get(context.Background())
	if ok {
		t.Error("should not find consent in empty context")
	}
}

func TestMustGet(t *testing.T) {
	ctx := Set(context.Background(), Consent{UserID: "user-1"})
	got := MustGet(ctx)
	if got.UserID != "user-1" {
		t.Errorf("UserID = %q", got.UserID)
	}
}

func TestMustGet_EmptyContext(t *testing.T) {
	got := MustGet(context.Background())
	if got.UserID != "" {
		t.Error("should return zero value")
	}
}

func TestHasScope_Context(t *testing.T) {
	ctx := Set(context.Background(), Consent{
		Scopes: []string{ScopeNIK, ScopeEmail},
	})

	if !HasScope(ctx, ScopeNIK) {
		t.Error("should have NIK")
	}
	if HasScope(ctx, ScopePhone) {
		t.Error("should not have phone")
	}
}

func TestHasScope_ExpiredConsent(t *testing.T) {
	ctx := Set(context.Background(), Consent{
		Scopes:    []string{ScopeNIK},
		ExpiresAt: time.Now().Add(-1 * time.Hour),
	})

	if HasScope(ctx, ScopeNIK) {
		t.Error("expired consent should not have scope")
	}
}

func TestHasScope_EmptyContext(t *testing.T) {
	if HasScope(context.Background(), ScopeNIK) {
		t.Error("empty context should not have scope")
	}
}

func TestHasAllScopes_Context(t *testing.T) {
	ctx := Set(context.Background(), Consent{
		Scopes: []string{ScopeNIK, ScopeEmail, ScopeName},
	})

	if !HasAllScopes(ctx, ScopeNIK, ScopeEmail) {
		t.Error("should have both")
	}
	if HasAllScopes(ctx, ScopeNIK, ScopePhone) {
		t.Error("should not have phone")
	}
}

func TestHasAllScopes_ExpiredConsent(t *testing.T) {
	ctx := Set(context.Background(), Consent{
		Scopes:    []string{ScopeNIK, ScopeEmail},
		ExpiresAt: time.Now().Add(-1 * time.Hour),
	})

	if HasAllScopes(ctx, ScopeNIK) {
		t.Error("expired consent should not pass")
	}
}

func TestSortedScopes(t *testing.T) {
	ctx := Set(context.Background(), Consent{
		Scopes: []string{ScopePhone, ScopeNIK, ScopeEmail, ScopeName},
	})

	scopes := SortedScopes(ctx)
	if len(scopes) != 4 {
		t.Fatalf("len = %d", len(scopes))
	}
	if scopes[0] != ScopeEmail {
		t.Errorf("[0] = %q, want email", scopes[0])
	}
}

func TestSortedScopes_EmptyContext(t *testing.T) {
	scopes := SortedScopes(context.Background())
	if scopes != nil {
		t.Error("should be nil for empty context")
	}
}

func TestScopeConstants(t *testing.T) {
	scopes := []string{
		ScopeNIK, ScopeName, ScopeEmail, ScopePhone, ScopeAddress,
		ScopeBirthDate, ScopePhoto, ScopeReligion, ScopeMarital,
		ScopeBloodType, ScopeFamily,
	}
	seen := make(map[string]bool)
	for _, s := range scopes {
		if s == "" {
			t.Error("scope constant should not be empty")
		}
		if seen[s] {
			t.Errorf("duplicate scope constant: %q", s)
		}
		seen[s] = true
	}
}
