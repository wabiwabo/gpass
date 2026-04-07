package oauth2

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// TestFetchDiscovery_NetworkAndStatusBranches pins the discovery endpoint
// non-200 path and the JSON decode error path. The happy path is covered
// elsewhere.
func TestFetchDiscovery_NetworkAndStatusBranches(t *testing.T) {
	// 500 from issuer
	srv500 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(500)
	}))
	defer srv500.Close()
	if _, err := FetchDiscovery(context.Background(), srv500.URL); err == nil || !strings.Contains(err.Error(), "status 500") {
		t.Errorf("500 case: %v", err)
	}

	// Garbage body
	srvBad := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte("not json"))
	}))
	defer srvBad.Close()
	if _, err := FetchDiscovery(context.Background(), srvBad.URL); err == nil || !strings.Contains(err.Error(), "decoding") {
		t.Errorf("bad json: %v", err)
	}

	// Unreachable
	if _, err := FetchDiscovery(context.Background(), "http://127.0.0.1:1"); err == nil {
		t.Error("unreachable should error")
	}
}

// TestExchange_NetworkAndDecodeBranches pins the non-200 and JSON decode
// error paths in TokenExchanger.Exchange.
func TestExchange_NetworkAndDecodeBranches(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify the form was set up correctly.
		_ = r.ParseForm()
		if r.Form.Get("grant_type") != GrantTypeTokenExchange {
			t.Errorf("grant_type = %q", r.Form.Get("grant_type"))
		}
		if r.Form.Get("audience") != "aud" {
			t.Errorf("audience missing")
		}
		w.WriteHeader(401)
		w.Write([]byte(`{"error":"invalid_client"}`))
	}))
	defer srv.Close()

	ex := NewTokenExchanger(srv.URL, "id", "secret")
	_, err := ex.Exchange(context.Background(), TokenExchangeRequest{
		SubjectToken:       "tok",
		SubjectTokenType:   "urn:ietf:params:oauth:token-type:access_token",
		RequestedTokenType: "urn:ietf:params:oauth:token-type:jwt",
		Audience:           "aud",
		Scope:              "read",
		Resource:           "https://api.example.com",
	})
	if err == nil || !strings.Contains(err.Error(), "status 401") {
		t.Errorf("401: %v", err)
	}

	// Bad JSON on 200.
	srvBad := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte("not json"))
	}))
	defer srvBad.Close()
	ex2 := NewTokenExchanger(srvBad.URL, "id", "secret")
	_, err = ex2.Exchange(context.Background(), TokenExchangeRequest{
		SubjectToken:     "x",
		SubjectTokenType: "y",
	})
	if err == nil || !strings.Contains(err.Error(), "decoding") {
		t.Errorf("bad json: %v", err)
	}
}

// TestIntrospect_NetworkAndDecodeBranches pins the non-200 and JSON
// decode error paths in Introspector.Introspect.
func TestIntrospect_NetworkAndDecodeBranches(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(503)
	}))
	defer srv.Close()
	in := NewIntrospector(srv.URL, "id", "sec")
	if _, err := in.Introspect(context.Background(), "tok"); err == nil || !strings.Contains(err.Error(), "status 503") {
		t.Errorf("503: %v", err)
	}

	srvBad := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte("not json"))
	}))
	defer srvBad.Close()
	in2 := NewIntrospector(srvBad.URL, "id", "sec")
	if _, err := in2.Introspect(context.Background(), "tok"); err == nil || !strings.Contains(err.Error(), "decoding") {
		t.Errorf("bad json: %v", err)
	}
}

// TestJWKS_RefreshErrorBranches pins the non-200 and JSON decode error
// paths in JWKSClient.Refresh.
func TestJWKS_RefreshErrorBranches(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(500)
	}))
	defer srv.Close()
	c := NewJWKSClient(srv.URL, time.Minute)
	if err := c.Refresh(context.Background()); err == nil || !strings.Contains(err.Error(), "status 500") {
		t.Errorf("500: %v", err)
	}

	srvBad := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte("not json"))
	}))
	defer srvBad.Close()
	c2 := NewJWKSClient(srvBad.URL, time.Minute)
	if err := c2.Refresh(context.Background()); err == nil || !strings.Contains(err.Error(), "decoding") {
		t.Errorf("bad json: %v", err)
	}

	// Unreachable
	c3 := NewJWKSClient("http://127.0.0.1:1", time.Minute)
	if err := c3.Refresh(context.Background()); err == nil {
		t.Error("unreachable should error")
	}
}
