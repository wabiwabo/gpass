package keycloak

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

// fakeKeycloak returns a httptest server that mimics the master-realm
// token endpoint and the create-user endpoint with configurable status
// codes for each call.
func fakeKeycloak(t *testing.T, tokenStatus, createStatus int, location string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/openid-connect/token") {
			if tokenStatus != 200 {
				w.WriteHeader(tokenStatus)
				w.Write([]byte(`{"error":"unauthorized"}`))
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"access_token":"test-token-xyz","expires_in":300}`))
			return
		}
		if strings.HasSuffix(r.URL.Path, "/users") {
			if r.Header.Get("Authorization") != "Bearer test-token-xyz" {
				t.Errorf("auth header not propagated: %q", r.Header.Get("Authorization"))
			}
			if location != "" {
				w.Header().Set("Location", location)
			}
			w.WriteHeader(createStatus)
			return
		}
	}))
	return srv
}

// TestCreateUser_HappyPath pins the full token-fetch → create-user →
// extract-id-from-Location loop.
func TestCreateUser_HappyPath(t *testing.T) {
	srv := fakeKeycloak(t, 200, 201, "https://kc.example/admin/realms/garudapass/users/abc-123")
	defer srv.Close()

	c := NewAdminClient(srv.URL, "admin", "secret", "garudapass")
	id, err := c.CreateUser(context.Background(), CreateUserRequest{
		Username: "alice", Email: "alice@example.com", Enabled: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if id != "abc-123" {
		t.Errorf("id = %q, want abc-123", id)
	}
}

// TestCreateUser_Conflict409 pins the user-already-exists branch.
func TestCreateUser_Conflict409(t *testing.T) {
	srv := fakeKeycloak(t, 200, http.StatusConflict, "")
	defer srv.Close()
	c := NewAdminClient(srv.URL, "admin", "secret", "r")
	_, err := c.CreateUser(context.Background(), CreateUserRequest{Username: "x", Enabled: true})
	if err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Errorf("err = %v", err)
	}
}

// TestCreateUser_Non2xxStatus pins the generic non-2xx branch.
func TestCreateUser_Non2xxStatus(t *testing.T) {
	srv := fakeKeycloak(t, 200, 500, "")
	defer srv.Close()
	c := NewAdminClient(srv.URL, "admin", "secret", "r")
	_, err := c.CreateUser(context.Background(), CreateUserRequest{Username: "x", Enabled: true})
	if err == nil || !strings.Contains(err.Error(), "status 500") {
		t.Errorf("err = %v", err)
	}
}

// TestCreateUser_MissingLocationHeader pins the empty-Location branch.
func TestCreateUser_MissingLocationHeader(t *testing.T) {
	srv := fakeKeycloak(t, 200, 201, "") // 201 but no Location
	defer srv.Close()
	c := NewAdminClient(srv.URL, "admin", "secret", "r")
	_, err := c.CreateUser(context.Background(), CreateUserRequest{Username: "x", Enabled: true})
	if err == nil || !strings.Contains(err.Error(), "Location") {
		t.Errorf("err = %v", err)
	}
}

// TestGetToken_Failed pins the token-endpoint non-200 branch.
func TestGetToken_Failed(t *testing.T) {
	srv := fakeKeycloak(t, 401, 0, "")
	defer srv.Close()
	c := NewAdminClient(srv.URL, "admin", "wrong", "r")
	_, err := c.CreateUser(context.Background(), CreateUserRequest{Username: "x", Enabled: true})
	if err == nil || !strings.Contains(err.Error(), "get admin token") {
		t.Errorf("err = %v", err)
	}
}

// TestGetToken_CachedAcrossCalls pins the cache-hit branch — second
// CreateUser call must NOT hit the token endpoint.
func TestGetToken_CachedAcrossCalls(t *testing.T) {
	var tokenCalls atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/openid-connect/token") {
			tokenCalls.Add(1)
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"access_token":"tok","expires_in":300}`))
			return
		}
		w.Header().Set("Location", "/admin/realms/r/users/uid-1")
		w.WriteHeader(201)
	}))
	defer srv.Close()

	c := NewAdminClient(srv.URL, "admin", "s", "r")
	for i := 0; i < 3; i++ {
		if _, err := c.CreateUser(context.Background(), CreateUserRequest{Username: "x", Enabled: true}); err != nil {
			t.Fatalf("call %d: %v", i, err)
		}
	}
	if tokenCalls.Load() != 1 {
		t.Errorf("token endpoint called %d times, want 1 (cache miss)", tokenCalls.Load())
	}
}
