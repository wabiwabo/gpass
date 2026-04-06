package handler

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestServiceProxy_CorrectBackend(t *testing.T) {
	identity := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("identity-service"))
	}))
	defer identity.Close()

	consent := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("consent-service"))
	}))
	defer consent.Close()

	proxy := NewServiceProxy(map[string]string{
		"/api/v1/identity": identity.URL,
		"/api/v1/consent":  consent.URL,
	})

	tests := []struct {
		name     string
		path     string
		wantBody string
	}{
		{"identity route", "/api/v1/identity/users", "identity-service"},
		{"consent route", "/api/v1/consent/grants", "consent-service"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			rec := httptest.NewRecorder()
			proxy.ServeHTTP(rec, req)

			body := rec.Body.String()
			if body != tt.wantBody {
				t.Errorf("got body %q, want %q", body, tt.wantBody)
			}
			if rec.Code != http.StatusOK {
				t.Errorf("got status %d, want %d", rec.Code, http.StatusOK)
			}
		})
	}
}

func TestServiceProxy_ForwardsXUserID(t *testing.T) {
	var gotUserID string
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUserID = r.Header.Get("X-User-ID")
		w.WriteHeader(http.StatusOK)
	}))
	defer backend.Close()

	proxy := NewServiceProxy(map[string]string{
		"/api/v1/identity": backend.URL,
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/identity/me", nil)
	req.Header.Set("X-User-ID", "user-123")
	rec := httptest.NewRecorder()
	proxy.ServeHTTP(rec, req)

	if gotUserID != "user-123" {
		t.Errorf("got X-User-ID %q, want %q", gotUserID, "user-123")
	}
}

func TestServiceProxy_ForwardsXRequestId(t *testing.T) {
	var gotRequestID string
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotRequestID = r.Header.Get("X-Request-Id")
		w.WriteHeader(http.StatusOK)
	}))
	defer backend.Close()

	proxy := NewServiceProxy(map[string]string{
		"/api/v1/identity": backend.URL,
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/identity/me", nil)
	req.Header.Set("X-Request-Id", "req-abc-123")
	rec := httptest.NewRecorder()
	proxy.ServeHTTP(rec, req)

	if gotRequestID != "req-abc-123" {
		t.Errorf("got X-Request-Id %q, want %q", gotRequestID, "req-abc-123")
	}
}

func TestServiceProxy_UnknownPathReturns404(t *testing.T) {
	proxy := NewServiceProxy(map[string]string{
		"/api/v1/identity": "http://localhost:9999",
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/unknown/resource", nil)
	rec := httptest.NewRecorder()
	proxy.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("got status %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestServiceProxy_BackendDownReturns502(t *testing.T) {
	// Use a URL that will fail to connect
	proxy := NewServiceProxy(map[string]string{
		"/api/v1/identity": "http://127.0.0.1:1", // port 1 should be unreachable
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/identity/me", nil)
	rec := httptest.NewRecorder()
	proxy.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadGateway {
		t.Errorf("got status %d, want %d", rec.Code, http.StatusBadGateway)
	}
}

func TestServiceProxy_ResponseBodyAndStatusForwarded(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Custom-Header", "custom-value")
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(`{"id":"new-resource"}`))
	}))
	defer backend.Close()

	proxy := NewServiceProxy(map[string]string{
		"/api/v1/identity": backend.URL,
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/identity/users", strings.NewReader(`{"name":"test"}`))
	rec := httptest.NewRecorder()
	proxy.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Errorf("got status %d, want %d", rec.Code, http.StatusCreated)
	}

	body, _ := io.ReadAll(rec.Body)
	if string(body) != `{"id":"new-resource"}` {
		t.Errorf("got body %q, want %q", string(body), `{"id":"new-resource"}`)
	}

	if got := rec.Header().Get("X-Custom-Header"); got != "custom-value" {
		t.Errorf("got X-Custom-Header %q, want %q", got, "custom-value")
	}
}

func TestServiceProxy_DefaultRoutes(t *testing.T) {
	proxy := NewServiceProxy(nil)

	expectedRoutes := []string{
		"/api/v1/identity",
		"/api/v1/consent",
		"/api/v1/corp",
		"/api/v1/sign",
		"/api/v1/portal",
		"/api/v1/audit",
		"/api/v1/notify",
	}

	for _, route := range expectedRoutes {
		if _, ok := proxy.routes[route]; !ok {
			t.Errorf("missing default route %s", route)
		}
	}
}
