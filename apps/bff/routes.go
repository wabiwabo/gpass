package main

import (
	"encoding/json"
	"net/http"

	"github.com/garudapass/gpass/apps/bff/handler"
)

// Route defines a BFF API route with metadata.
type Route struct {
	Method      string
	Path        string
	Handler     http.HandlerFunc
	Description string
	Auth        string // "none", "session", "admin"
	RateLimit   int    // requests per minute, 0 = default
}

// RouteDeps contains all handler dependencies needed for route registration.
type RouteDeps struct {
	AuthHandler      *handler.AuthHandler
	SessionHandler   *handler.SessionHandler
	HealthAgg        *handler.HealthAggregator
	DashboardHandler *handler.DashboardHandler
	Proxy            *handler.ServiceProxy
	ReadinessHandler *handler.ReadinessHandler
}

// RegisterRoutes registers all BFF routes on the given mux.
// Returns the route registry for documentation.
func RegisterRoutes(mux *http.ServeMux, deps RouteDeps) []Route {
	routes := []Route{
		// Health endpoints — no auth required
		{
			Method:      "GET",
			Path:        "/health",
			Handler:     healthHandler,
			Description: "Basic health check for load balancers",
			Auth:        "none",
		},
		{
			Method:      "GET",
			Path:        "/ready",
			Handler:     deps.ReadinessHandler.ServeHTTP,
			Description: "Deep readiness check for Kubernetes probes",
			Auth:        "none",
		},
		{
			Method:      "GET",
			Path:        "/health/all",
			Handler:     deps.HealthAgg.ServeHTTP,
			Description: "Aggregated health of all platform services",
			Auth:        "none",
		},

		// Auth endpoints — rate limited, no CSRF
		{
			Method:      "GET",
			Path:        "/auth/login",
			Handler:     deps.AuthHandler.Login,
			Description: "Initiate OAuth2 PKCE login flow",
			Auth:        "none",
			RateLimit:   30,
		},
		{
			Method:      "GET",
			Path:        "/auth/callback",
			Handler:     deps.AuthHandler.Callback,
			Description: "OAuth2 callback to exchange code for tokens",
			Auth:        "none",
			RateLimit:   30,
		},
		{
			Method:      "POST",
			Path:        "/auth/logout",
			Handler:     deps.AuthHandler.Logout,
			Description: "Logout and destroy session",
			Auth:        "none",
			RateLimit:   30,
		},
		{
			Method:      "GET",
			Path:        "/auth/session",
			Handler:     deps.SessionHandler.GetSession,
			Description: "Get current session info without CSRF",
			Auth:        "none",
			RateLimit:   30,
		},

		// API endpoints — require session + CSRF
		{
			Method:      "GET",
			Path:        "/api/v1/me",
			Handler:     deps.SessionHandler.GetSession,
			Description: "Get authenticated user profile",
			Auth:        "session",
		},

		// Admin endpoints — require admin role
		{
			Method:      "GET",
			Path:        "/api/v1/admin/dashboard",
			Handler:     deps.DashboardHandler.GetDashboard,
			Description: "Platform admin dashboard with service status",
			Auth:        "admin",
		},
		{
			Method:      "GET",
			Path:        "/api/v1/admin/health",
			Handler:     deps.HealthAgg.ServeHTTP,
			Description: "Admin view of aggregated platform health",
			Auth:        "admin",
		},
		{
			Method:      "GET",
			Path:        "/api/v1/admin/routes",
			Handler:     routeListHandler(nil), // placeholder, replaced below
			Description: "List all registered routes and metadata",
			Auth:        "admin",
		},
	}

	// Replace the route list handler with one that has the full route list
	for i, rt := range routes {
		if rt.Path == "/api/v1/admin/routes" {
			routes[i].Handler = routeListHandler(routes)
			break
		}
	}

	// Register on the mux
	for _, rt := range routes {
		mux.HandleFunc(rt.Method+" "+rt.Path, rt.Handler)
	}

	return routes
}

// healthHandler is the basic health check handler.
func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"ok"}`))
}

// routeListHandler returns a handler that serves the route registry as JSON.
func routeListHandler(routes []Route) http.HandlerFunc {
	type routeInfo struct {
		Method      string `json:"method"`
		Path        string `json:"path"`
		Description string `json:"description"`
		Auth        string `json:"auth"`
		RateLimit   int    `json:"rate_limit,omitempty"`
	}

	return func(w http.ResponseWriter, r *http.Request) {
		infos := make([]routeInfo, len(routes))
		for i, rt := range routes {
			infos[i] = routeInfo{
				Method:      rt.Method,
				Path:        rt.Path,
				Description: rt.Description,
				Auth:        rt.Auth,
				RateLimit:   rt.RateLimit,
			}
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(infos)
	}
}
