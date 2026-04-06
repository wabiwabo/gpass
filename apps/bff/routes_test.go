package main

import (
	"context"
	"net/http"
	"testing"

	"github.com/garudapass/gpass/apps/bff/handler"
	"github.com/garudapass/gpass/apps/bff/session"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func testRouteDeps(t *testing.T) RouteDeps {
	t.Helper()

	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { rdb.Close() })

	store, err := session.NewRedisStore(rdb, nil)
	if err != nil {
		t.Fatalf("failed to create session store: %v", err)
	}

	authHandler := handler.NewAuthHandler(handler.AuthConfig{
		IssuerURL:   "http://localhost:8080/realms/test",
		ClientID:    "test",
		RedirectURI: "http://localhost:3000/auth/callback",
		FrontendURL: "http://localhost:3000",
	}, store)

	sessionHandler := handler.NewSessionHandler(store)
	healthAgg := handler.NewHealthAggregatorWithServices("test", map[string]string{})
	dashboardHandler := handler.NewDashboardHandler(healthAgg, "test")
	proxy := handler.NewServiceProxy(map[string]string{})
	readinessHandler := handler.NewReadinessHandler(
		handler.ReadinessCheck{Name: "redis", Critical: true, Check: func(ctx context.Context) error { return nil }},
	)

	return RouteDeps{
		AuthHandler:      authHandler,
		SessionHandler:   sessionHandler,
		HealthAgg:        healthAgg,
		DashboardHandler: dashboardHandler,
		Proxy:            proxy,
		ReadinessHandler: readinessHandler,
	}
}

func TestRegisterRoutes_RouteCount(t *testing.T) {
	mux := http.NewServeMux()
	deps := testRouteDeps(t)
	routes := RegisterRoutes(mux, deps)

	// We expect at least 11 routes
	if len(routes) < 11 {
		t.Errorf("expected at least 11 routes, got %d", len(routes))
	}
}

func TestRegisterRoutes_HealthEndpointsAuthNone(t *testing.T) {
	mux := http.NewServeMux()
	deps := testRouteDeps(t)
	routes := RegisterRoutes(mux, deps)

	healthPaths := map[string]bool{
		"/health":     false,
		"/ready":      false,
		"/health/all": false,
	}

	for _, rt := range routes {
		if _, ok := healthPaths[rt.Path]; ok {
			if rt.Auth != "none" {
				t.Errorf("health endpoint %s should have auth=none, got %s", rt.Path, rt.Auth)
			}
			healthPaths[rt.Path] = true
		}
	}

	for path, found := range healthPaths {
		if !found {
			t.Errorf("health endpoint %s not found in routes", path)
		}
	}
}

func TestRegisterRoutes_AdminEndpointsAuthAdmin(t *testing.T) {
	mux := http.NewServeMux()
	deps := testRouteDeps(t)
	routes := RegisterRoutes(mux, deps)

	adminFound := 0
	for _, rt := range routes {
		if rt.Auth == "admin" {
			adminFound++
			if rt.Path != "/api/v1/admin/dashboard" &&
				rt.Path != "/api/v1/admin/health" &&
				rt.Path != "/api/v1/admin/routes" {
				t.Errorf("unexpected admin route: %s", rt.Path)
			}
		}
	}

	if adminFound < 3 {
		t.Errorf("expected at least 3 admin routes, got %d", adminFound)
	}
}

func TestRegisterRoutes_APIEndpointsAuthSession(t *testing.T) {
	mux := http.NewServeMux()
	deps := testRouteDeps(t)
	routes := RegisterRoutes(mux, deps)

	for _, rt := range routes {
		if rt.Path == "/api/v1/me" {
			if rt.Auth != "session" {
				t.Errorf("API endpoint %s should have auth=session, got %s", rt.Path, rt.Auth)
			}
			return
		}
	}
	t.Error("expected /api/v1/me route not found")
}

func TestRegisterRoutes_NoDuplicatePaths(t *testing.T) {
	mux := http.NewServeMux()
	deps := testRouteDeps(t)
	routes := RegisterRoutes(mux, deps)

	seen := make(map[string]bool)
	for _, rt := range routes {
		key := rt.Method + " " + rt.Path
		if seen[key] {
			t.Errorf("duplicate route: %s %s", rt.Method, rt.Path)
		}
		seen[key] = true
	}
}

func TestRegisterRoutes_DescriptionsNonEmpty(t *testing.T) {
	mux := http.NewServeMux()
	deps := testRouteDeps(t)
	routes := RegisterRoutes(mux, deps)

	for _, rt := range routes {
		if rt.Description == "" {
			t.Errorf("route %s %s has empty description", rt.Method, rt.Path)
		}
	}
}
