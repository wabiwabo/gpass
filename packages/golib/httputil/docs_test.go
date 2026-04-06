package httputil

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRegisterAndAll(t *testing.T) {
	r := NewRouteRegistry()
	r.Register("GET", "/api/users", "List users", "required")
	r.Register("POST", "/api/users", "Create user", "required")

	routes := r.All()
	if len(routes) != 2 {
		t.Fatalf("expected 2 routes, got %d", len(routes))
	}
	if routes[0].Method != "GET" {
		t.Errorf("expected GET, got %q", routes[0].Method)
	}
	if routes[0].Path != "/api/users" {
		t.Errorf("expected /api/users, got %q", routes[0].Path)
	}
	if routes[0].Description != "List users" {
		t.Errorf("expected 'List users', got %q", routes[0].Description)
	}
	if routes[0].Auth != "required" {
		t.Errorf("expected 'required', got %q", routes[0].Auth)
	}
}

func TestHandlerServesJSON(t *testing.T) {
	r := NewRouteRegistry()
	r.Register("GET", "/health", "Health check", "none")
	r.Register("GET", "/api/docs", "API documentation", "none")

	req := httptest.NewRequest(http.MethodGet, "/api/docs", nil)
	rec := httptest.NewRecorder()

	r.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}
	ct := rec.Header().Get("Content-Type")
	if ct != "application/json; charset=utf-8" {
		t.Errorf("expected JSON content type, got %q", ct)
	}

	var routes []RouteInfo
	if err := json.NewDecoder(rec.Body).Decode(&routes); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}
	if len(routes) != 2 {
		t.Fatalf("expected 2 routes, got %d", len(routes))
	}
}

func TestGroupByPath(t *testing.T) {
	r := NewRouteRegistry()
	r.Register("GET", "/api/users", "List users", "required")
	r.Register("POST", "/api/users", "Create user", "required")
	r.Register("GET", "/api/users/:id", "Get user", "required")
	r.Register("GET", "/api/sessions", "List sessions", "required")
	r.Register("GET", "/health", "Health check", "none")

	groups := r.GroupByPath()

	if len(groups["/api/users"]) != 3 {
		t.Errorf("expected 3 routes in /api/users, got %d", len(groups["/api/users"]))
	}
	if len(groups["/api/sessions"]) != 1 {
		t.Errorf("expected 1 route in /api/sessions, got %d", len(groups["/api/sessions"]))
	}
	if len(groups["/health"]) != 1 {
		t.Errorf("expected 1 route in /health, got %d", len(groups["/health"]))
	}
}

func TestEmptyRegistry(t *testing.T) {
	r := NewRouteRegistry()

	routes := r.All()
	if len(routes) != 0 {
		t.Errorf("expected 0 routes, got %d", len(routes))
	}

	groups := r.GroupByPath()
	if len(groups) != 0 {
		t.Errorf("expected 0 groups, got %d", len(groups))
	}
}

func TestMultipleRoutesSamePathDifferentMethods(t *testing.T) {
	r := NewRouteRegistry()
	r.Register("GET", "/api/items", "List items", "required")
	r.Register("POST", "/api/items", "Create item", "required")
	r.Register("DELETE", "/api/items", "Delete all items", "required")

	routes := r.All()
	if len(routes) != 3 {
		t.Fatalf("expected 3 routes, got %d", len(routes))
	}

	methods := map[string]bool{}
	for _, route := range routes {
		methods[route.Method] = true
		if route.Path != "/api/items" {
			t.Errorf("expected path /api/items, got %q", route.Path)
		}
	}
	if !methods["GET"] || !methods["POST"] || !methods["DELETE"] {
		t.Errorf("expected GET, POST, DELETE methods, got %v", methods)
	}
}
