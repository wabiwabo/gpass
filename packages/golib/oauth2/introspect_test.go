package oauth2

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestIntrospect_ActiveToken(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if ct := r.Header.Get("Content-Type"); ct != "application/x-www-form-urlencoded" {
			t.Errorf("unexpected content-type: %s", ct)
		}

		// Verify basic auth credentials.
		user, pass, ok := r.BasicAuth()
		if !ok || user != "client-id" || pass != "client-secret" {
			t.Errorf("unexpected basic auth: %s/%s", user, pass)
		}

		if err := r.ParseForm(); err != nil {
			t.Fatalf("parsing form: %v", err)
		}
		if tok := r.FormValue("token"); tok != "valid-token" {
			t.Errorf("unexpected token: %s", tok)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(TokenInfo{
			Active:    true,
			Scope:     "openid profile",
			ClientID:  "some-client",
			Username:  "john",
			TokenType: "Bearer",
			ExpiresAt: 1700000000,
			IssuedAt:  1699999000,
			Subject:   "user-123",
			Audience:  "api",
			Issuer:    "https://auth.example.com",
		})
	}))
	defer server.Close()

	intro := NewIntrospector(server.URL, "client-id", "client-secret")
	info, err := intro.Introspect(context.Background(), "valid-token")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !info.Active {
		t.Error("expected active=true")
	}
	if info.Username != "john" {
		t.Errorf("expected username=john, got %s", info.Username)
	}
	if info.Subject != "user-123" {
		t.Errorf("expected sub=user-123, got %s", info.Subject)
	}
	if info.Scope != "openid profile" {
		t.Errorf("expected scope=openid profile, got %s", info.Scope)
	}
}

func TestIntrospect_InactiveToken(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(TokenInfo{Active: false})
	}))
	defer server.Close()

	intro := NewIntrospector(server.URL, "client-id", "client-secret")
	info, err := intro.Introspect(context.Background(), "expired-token")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info.Active {
		t.Error("expected active=false")
	}
}

func TestIntrospect_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	intro := NewIntrospector(server.URL, "client-id", "client-secret")
	_, err := intro.Introspect(context.Background(), "some-token")
	if err == nil {
		t.Fatal("expected error for server error response")
	}
}

func TestMockIntrospector_AddAndRetrieve(t *testing.T) {
	mock := NewMockIntrospector()
	mock.AddToken("test-token", &TokenInfo{
		Active:   true,
		Username: "testuser",
		Subject:  "sub-1",
	})

	info, err := mock.Introspect(context.Background(), "test-token")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !info.Active {
		t.Error("expected active=true")
	}
	if info.Username != "testuser" {
		t.Errorf("expected username=testuser, got %s", info.Username)
	}
}

func TestMockIntrospector_UnknownToken(t *testing.T) {
	mock := NewMockIntrospector()

	info, err := mock.Introspect(context.Background(), "unknown-token")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info.Active {
		t.Error("expected active=false for unknown token")
	}
}
