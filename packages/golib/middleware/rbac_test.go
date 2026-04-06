package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRequireRole_AdminCanAccessAdmin(t *testing.T) {
	handler := RequireRole(RoleAdmin)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	r := httptest.NewRequest(http.MethodGet, "/admin", nil)
	r.Header.Set("X-User-Role", "admin")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestRequireRole_AdminCanAccessUserEndpoint(t *testing.T) {
	handler := RequireRole(RoleUser)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	r := httptest.NewRequest(http.MethodGet, "/profile", nil)
	r.Header.Set("X-User-Role", "admin")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200 (admin accessing user endpoint), got %d", w.Code)
	}
}

func TestRequireRole_UserCannotAccessAdmin(t *testing.T) {
	handler := RequireRole(RoleAdmin)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("next handler should not be called")
	}))

	r := httptest.NewRequest(http.MethodGet, "/admin", nil)
	r.Header.Set("X-User-Role", "user")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", w.Code)
	}
}

func TestRequireRole_MissingRoleHeader(t *testing.T) {
	handler := RequireRole(RoleUser)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("next handler should not be called")
	}))

	r := httptest.NewRequest(http.MethodGet, "/profile", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestRequireRole_ServiceCanAccessUserEndpoint(t *testing.T) {
	handler := RequireRole(RoleUser)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	r := httptest.NewRequest(http.MethodGet, "/profile", nil)
	r.Header.Set("X-User-Role", "service")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200 (service accessing user endpoint), got %d", w.Code)
	}
}

func TestHasRole_DirectMatch(t *testing.T) {
	if !HasRole(RoleUser, RoleUser) {
		t.Error("expected user to match user directly")
	}
	if !HasRole(RoleAdmin, RoleAdmin) {
		t.Error("expected admin to match admin directly")
	}
	if !HasRole(RoleService, RoleService) {
		t.Error("expected service to match service directly")
	}
}

func TestHasRole_ViaHierarchy(t *testing.T) {
	if !HasRole(RoleAdmin, RoleUser) {
		t.Error("expected admin to satisfy user via hierarchy")
	}
	if !HasRole(RoleAdmin, RoleDeveloper) {
		t.Error("expected admin to satisfy developer via hierarchy")
	}
	if !HasRole(RoleAdmin, RoleService) {
		t.Error("expected admin to satisfy service via hierarchy")
	}
	if !HasRole(RoleService, RoleUser) {
		t.Error("expected service to satisfy user via hierarchy")
	}
	if !HasRole(RoleDeveloper, RoleUser) {
		t.Error("expected developer to satisfy user via hierarchy")
	}
}

func TestHasRole_Negative(t *testing.T) {
	if HasRole(RoleUser, RoleAdmin) {
		t.Error("user should not satisfy admin")
	}
	if HasRole(RoleUser, RoleService) {
		t.Error("user should not satisfy service")
	}
	if HasRole(RoleUser, RoleDeveloper) {
		t.Error("user should not satisfy developer")
	}
	if HasRole(RoleDeveloper, RoleAdmin) {
		t.Error("developer should not satisfy admin")
	}
	if HasRole(RoleService, RoleAdmin) {
		t.Error("service should not satisfy admin")
	}
	if HasRole(RoleDeveloper, RoleService) {
		t.Error("developer should not satisfy service")
	}
	if HasRole(RoleService, RoleDeveloper) {
		t.Error("service should not satisfy developer")
	}
}
