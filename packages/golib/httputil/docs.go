package httputil

import (
	"encoding/json"
	"net/http"
	"strings"
	"sync"
)

// RouteInfo describes a registered HTTP route.
type RouteInfo struct {
	Method      string `json:"method"`
	Path        string `json:"path"`
	Description string `json:"description,omitempty"`
	Auth        string `json:"auth,omitempty"`
}

// RouteRegistry tracks all routes for documentation.
type RouteRegistry struct {
	routes []RouteInfo
	mu     sync.RWMutex
}

// NewRouteRegistry creates a new RouteRegistry.
func NewRouteRegistry() *RouteRegistry {
	return &RouteRegistry{}
}

// Register adds a route to the registry.
func (r *RouteRegistry) Register(method, path, description, auth string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.routes = append(r.routes, RouteInfo{
		Method:      method,
		Path:        path,
		Description: description,
		Auth:        auth,
	})
}

// All returns all registered routes.
func (r *RouteRegistry) All() []RouteInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]RouteInfo, len(r.routes))
	copy(result, r.routes)
	return result
}

// Handler returns an HTTP handler that serves route documentation as JSON.
func (r *RouteRegistry) Handler() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(r.All())
	}
}

// GroupByPath groups routes by their first path segment.
// For example, "/api/users" and "/api/users/:id" both group under "/api/users".
func (r *RouteRegistry) GroupByPath() map[string][]RouteInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()

	groups := make(map[string][]RouteInfo)
	for _, route := range r.routes {
		prefix := groupPrefix(route.Path)
		groups[prefix] = append(groups[prefix], route)
	}
	return groups
}

// groupPrefix extracts the first two path segments as a group key.
// "/api/users/:id" -> "/api/users"
// "/health" -> "/health"
func groupPrefix(path string) string {
	path = strings.TrimPrefix(path, "/")
	parts := strings.SplitN(path, "/", 3)
	if len(parts) <= 1 {
		return "/" + path
	}
	return "/" + parts[0] + "/" + parts[1]
}
