package tenantctx

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestFromContext(t *testing.T) {
	ctx := WithTenant(context.Background(), "tenant-1")
	id, ok := FromContext(ctx)
	if !ok || id != "tenant-1" {
		t.Errorf("got %q, ok=%v", id, ok)
	}
}

func TestFromContext_Missing(t *testing.T) {
	_, ok := FromContext(context.Background())
	if ok {
		t.Error("should return false")
	}
}

func TestMustFromContext(t *testing.T) {
	ctx := WithTenant(context.Background(), "tenant-2")
	id := MustFromContext(ctx)
	if id != "tenant-2" {
		t.Errorf("got %q", id)
	}
}

func TestMustFromContext_Panic(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("should panic")
		}
	}()
	MustFromContext(context.Background())
}

func TestMiddleware_ExtractsFromHeader(t *testing.T) {
	var captured string
	handler := Middleware(DefaultConfig())(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		captured, _ = FromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Tenant-ID", "acme")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if captured != "acme" {
		t.Errorf("captured: got %q", captured)
	}
	if w.Header().Get("X-Tenant-ID") != "acme" {
		t.Error("should echo tenant in response")
	}
}

func TestMiddleware_Required_Missing(t *testing.T) {
	handler := Middleware(DefaultConfig())(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("should not be called")
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("missing required: got %d", w.Code)
	}
}

func TestMiddleware_Optional_Missing(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Required = false

	handler := Middleware(cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("optional missing: got %d", w.Code)
	}
}

func TestMiddleware_AllowedTenants(t *testing.T) {
	cfg := Config{
		HeaderName:     "X-Tenant-ID",
		Required:       true,
		AllowedTenants: map[string]bool{"acme": true, "globex": true},
	}

	handler := Middleware(cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Allowed tenant.
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Tenant-ID", "acme")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("allowed: got %d", w.Code)
	}

	// Disallowed tenant.
	req = httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Tenant-ID", "unknown")
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Errorf("disallowed: got %d", w.Code)
	}
}

func TestMiddleware_CustomValidator(t *testing.T) {
	cfg := Config{
		HeaderName: "X-Tenant-ID",
		Required:   true,
		Validator: func(id string) error {
			if len(id) < 3 {
				return fmt.Errorf("tenant ID too short")
			}
			return nil
		},
	}

	handler := Middleware(cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Valid.
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Tenant-ID", "acme")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("valid: got %d", w.Code)
	}

	// Invalid.
	req = httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Tenant-ID", "ab")
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Errorf("invalid: got %d", w.Code)
	}
}

func TestMiddleware_Whitespace(t *testing.T) {
	handler := Middleware(DefaultConfig())(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("whitespace-only should be treated as missing")
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Tenant-ID", "   ")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("whitespace: got %d", w.Code)
	}
}
