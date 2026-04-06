package tenant

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRegisterAndGet(t *testing.T) {
	s := NewStore()
	tenant := Tenant{ID: "t1", Name: "Test Tenant", Environment: "sandbox", Tier: "free"}
	if err := s.Register(tenant); err != nil {
		t.Fatalf("Register failed: %v", err)
	}
	got, err := s.Get("t1")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if got.ID != "t1" || got.Name != "Test Tenant" {
		t.Errorf("unexpected tenant: %+v", got)
	}
}

func TestDuplicateRegistration(t *testing.T) {
	s := NewStore()
	tenant := Tenant{ID: "t1", Name: "Tenant"}
	if err := s.Register(tenant); err != nil {
		t.Fatalf("first Register failed: %v", err)
	}
	err := s.Register(tenant)
	if err != ErrDuplicate {
		t.Errorf("expected ErrDuplicate, got %v", err)
	}
}

func TestGetNotFound(t *testing.T) {
	s := NewStore()
	_, err := s.Get("nonexistent")
	if err != ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestListReturnsAllTenants(t *testing.T) {
	s := NewStore()
	_ = s.Register(Tenant{ID: "b", Name: "B"})
	_ = s.Register(Tenant{ID: "a", Name: "A"})
	_ = s.Register(Tenant{ID: "c", Name: "C"})

	list := s.List()
	if len(list) != 3 {
		t.Fatalf("expected 3 tenants, got %d", len(list))
	}
	// Should be sorted by ID
	if list[0].ID != "a" || list[1].ID != "b" || list[2].ID != "c" {
		t.Errorf("tenants not sorted: %v, %v, %v", list[0].ID, list[1].ID, list[2].ID)
	}
}

func TestUpdateModifiesConfig(t *testing.T) {
	s := NewStore()
	_ = s.Register(Tenant{ID: "t1", Name: "Tenant", Config: map[string]string{"key1": "val1"}})

	err := s.Update("t1", map[string]string{"key2": "val2"})
	if err != nil {
		t.Fatalf("Update failed: %v", err)
	}
	got, _ := s.Get("t1")
	if got.Config["key1"] != "val1" {
		t.Error("existing config lost")
	}
	if got.Config["key2"] != "val2" {
		t.Error("new config not applied")
	}
}

func TestUpdateNotFound(t *testing.T) {
	s := NewStore()
	err := s.Update("nonexistent", map[string]string{"k": "v"})
	if err != ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestMiddlewareExtractsTenantFromHeader(t *testing.T) {
	s := NewStore()
	_ = s.Register(Tenant{ID: "tenant-1", Name: "T1"})

	resolver := func(r *http.Request) string {
		return r.Header.Get("X-Tenant-ID")
	}

	var captured *Tenant
	handler := Middleware(s, resolver)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		captured, _ = FromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Tenant-ID", "tenant-1")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
	if captured == nil || captured.ID != "tenant-1" {
		t.Error("tenant not extracted from context")
	}
}

func TestMiddlewareUnknownTenantReturns403(t *testing.T) {
	s := NewStore()
	resolver := func(r *http.Request) string {
		return r.Header.Get("X-Tenant-ID")
	}

	handler := Middleware(s, resolver)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Tenant-ID", "unknown")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", rr.Code)
	}
}

func TestMiddlewareMissingTenantReturns403(t *testing.T) {
	s := NewStore()
	resolver := func(r *http.Request) string {
		return r.Header.Get("X-Tenant-ID")
	}

	handler := Middleware(s, resolver)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", rr.Code)
	}
}

func TestFromContext(t *testing.T) {
	s := NewStore()
	_ = s.Register(Tenant{ID: "ctx-test", Name: "CTX"})
	tenant, _ := s.Get("ctx-test")

	ctx := WithContext(t.Context(), tenant)
	got, ok := FromContext(ctx)
	if !ok {
		t.Fatal("FromContext returned false")
	}
	if got.ID != "ctx-test" {
		t.Errorf("expected ctx-test, got %s", got.ID)
	}
}

func TestFromContextMissing(t *testing.T) {
	_, ok := FromContext(t.Context())
	if ok {
		t.Error("expected false for empty context")
	}
}

func TestIsolationMiddlewareSetsHeader(t *testing.T) {
	s := NewStore()
	_ = s.Register(Tenant{ID: "iso-1", Name: "Isolated"})
	tenant, _ := s.Get("iso-1")

	var gotHeader string
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotHeader = r.Header.Get("X-Tenant-ID")
		w.WriteHeader(http.StatusOK)
	})

	handler := IsolationMiddleware(inner)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req = req.WithContext(WithContext(req.Context(), tenant))
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
	if gotHeader != "iso-1" {
		t.Errorf("expected X-Tenant-ID iso-1 in request, got %q", gotHeader)
	}
	if rr.Header().Get("X-Tenant-ID") != "iso-1" {
		t.Error("X-Tenant-ID not set in response header")
	}
}

func TestIsolationMiddlewareWithoutContext(t *testing.T) {
	handler := IsolationMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", rr.Code)
	}
}
