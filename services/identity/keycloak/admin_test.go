package keycloak

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCreateUser_Success(t *testing.T) {
	mux := http.NewServeMux()

	// Token endpoint
	mux.HandleFunc("POST /realms/master/protocol/openid-connect/token", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"access_token": "test-token",
			"expires_in":   300,
		})
	})

	// Create user endpoint
	mux.HandleFunc("POST /admin/realms/garudapass/users", func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if auth != "Bearer test-token" {
			t.Errorf("unexpected auth header: %s", auth)
		}
		w.Header().Set("Location", "http://localhost/admin/realms/garudapass/users/user-id-123")
		w.WriteHeader(http.StatusCreated)
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	client := NewAdminClient(server.URL, "admin", "admin", "garudapass")
	userID, err := client.CreateUser(context.Background(), CreateUserRequest{
		Username: "testuser",
		Email:    "test@example.com",
		Enabled:  true,
	})
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	if userID != "user-id-123" {
		t.Errorf("userID = %q, want %q", userID, "user-id-123")
	}
}

func TestCreateUser_Conflict(t *testing.T) {
	mux := http.NewServeMux()

	mux.HandleFunc("POST /realms/master/protocol/openid-connect/token", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"access_token": "test-token",
			"expires_in":   300,
		})
	})

	mux.HandleFunc("POST /admin/realms/garudapass/users", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusConflict)
		w.Write([]byte(`{"errorMessage":"User exists with same username"}`))
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	client := NewAdminClient(server.URL, "admin", "admin", "garudapass")
	_, err := client.CreateUser(context.Background(), CreateUserRequest{
		Username: "existinguser",
		Enabled:  true,
	})
	if err == nil {
		t.Fatal("expected error for conflict")
	}
}

func TestCreateUser_TokenCaching(t *testing.T) {
	tokenCalls := 0
	mux := http.NewServeMux()

	mux.HandleFunc("POST /realms/master/protocol/openid-connect/token", func(w http.ResponseWriter, r *http.Request) {
		tokenCalls++
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"access_token": "cached-token",
			"expires_in":   300,
		})
	})

	mux.HandleFunc("POST /admin/realms/garudapass/users", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Location", "http://localhost/admin/realms/garudapass/users/id-1")
		w.WriteHeader(http.StatusCreated)
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	client := NewAdminClient(server.URL, "admin", "admin", "garudapass")

	// Two calls should only fetch token once
	client.CreateUser(context.Background(), CreateUserRequest{Username: "u1", Enabled: true})
	client.CreateUser(context.Background(), CreateUserRequest{Username: "u2", Enabled: true})

	if tokenCalls != 1 {
		t.Errorf("token fetched %d times, want 1 (should be cached)", tokenCalls)
	}
}
