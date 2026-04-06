package mwtenant

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSetTenantID_TenantID(t *testing.T) {
	ctx := SetTenantID(context.Background(), "tenant-123")
	if TenantID(ctx) != "tenant-123" { t.Error("wrong tenant") }
}

func TestTenantID_Empty(t *testing.T) {
	if TenantID(context.Background()) != "" { t.Error("should be empty") }
}

func TestValidTenantID(t *testing.T) {
	valid := []string{"tenant-1", "my_tenant", "abc123", "a-b", "abc"}
	for _, id := range valid {
		if !ValidTenantID(id) { t.Errorf("%q should be valid", id) }
	}
	invalid := []string{"", "a", "ab", "-start", "end-", "has space", "a@b"}
	for _, id := range invalid {
		if ValidTenantID(id) { t.Errorf("%q should be invalid", id) }
	}
}

func TestMiddleware_Header(t *testing.T) {
	mw := Middleware(Config{Source: SourceHeader})
	var captured string
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		captured = TenantID(r.Context())
	}))

	req := httptest.NewRequest("GET", "/api", nil)
	req.Header.Set("X-Tenant-ID", "tenant-abc")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if captured != "tenant-abc" { t.Errorf("captured = %q", captured) }
}

func TestMiddleware_Required_Missing(t *testing.T) {
	mw := Middleware(Config{Required: true})
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("should not call handler")
	}))

	req := httptest.NewRequest("GET", "/api", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != 400 { t.Errorf("status = %d", w.Code) }
}

func TestMiddleware_InvalidFormat(t *testing.T) {
	mw := Middleware(Config{})
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("should not call handler")
	}))

	req := httptest.NewRequest("GET", "/api", nil)
	req.Header.Set("X-Tenant-ID", "-invalid")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != 400 { t.Errorf("status = %d", w.Code) }
}

func TestMiddleware_Validator_Rejects(t *testing.T) {
	mw := Middleware(Config{
		Validator: func(id string) bool { return id == "allowed" },
	})
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("should not call handler")
	}))

	req := httptest.NewRequest("GET", "/api", nil)
	req.Header.Set("X-Tenant-ID", "unknown-tenant")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != 404 { t.Errorf("status = %d", w.Code) }
}

func TestMiddleware_Optional(t *testing.T) {
	mw := Middleware(Config{Required: false})
	var called bool
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	}))

	req := httptest.NewRequest("GET", "/api", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if !called { t.Error("should call handler when optional") }
}

func TestMiddleware_PathSource(t *testing.T) {
	mw := Middleware(Config{Source: SourcePath, PathIndex: 1})
	var captured string
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		captured = TenantID(r.Context())
	}))

	req := httptest.NewRequest("GET", "/api/tenant-xyz/users", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if captured != "tenant-xyz" { t.Errorf("captured = %q", captured) }
}
