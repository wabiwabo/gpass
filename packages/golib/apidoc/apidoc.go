// Package apidoc provides runtime API documentation generation.
// Services register their endpoints, and the package serves a
// JSON discovery document listing all available operations.
package apidoc

import (
	"encoding/json"
	"net/http"
	"sort"
	"sync"
)

// Endpoint describes a single API endpoint.
type Endpoint struct {
	Method      string   `json:"method"`
	Path        string   `json:"path"`
	Description string   `json:"description"`
	Tags        []string `json:"tags,omitempty"`
	Auth        string   `json:"auth,omitempty"` // "none", "session", "api_key", "service"
	RateLimit   string   `json:"rate_limit,omitempty"`
}

// ServiceDoc holds API documentation for a service.
type ServiceDoc struct {
	Name        string     `json:"name"`
	Version     string     `json:"version"`
	Description string     `json:"description"`
	BasePath    string     `json:"base_path"`
	Endpoints   []Endpoint `json:"endpoints"`
}

// Registry manages API documentation.
type Registry struct {
	mu       sync.RWMutex
	name     string
	version  string
	desc     string
	basePath string
	endpoints []Endpoint
}

// NewRegistry creates an API documentation registry.
func NewRegistry(name, version, description string) *Registry {
	return &Registry{
		name:    name,
		version: version,
		desc:    description,
	}
}

// SetBasePath sets the API base path.
func (r *Registry) SetBasePath(path string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.basePath = path
}

// Register adds an endpoint to the documentation.
func (r *Registry) Register(endpoint Endpoint) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.endpoints = append(r.endpoints, endpoint)
}

// RegisterRoute is a convenience method for common registrations.
func (r *Registry) RegisterRoute(method, path, description, auth string, tags ...string) {
	r.Register(Endpoint{
		Method:      method,
		Path:        path,
		Description: description,
		Auth:        auth,
		Tags:        tags,
	})
}

// Doc returns the complete service documentation.
func (r *Registry) Doc() ServiceDoc {
	r.mu.RLock()
	defer r.mu.RUnlock()

	endpoints := make([]Endpoint, len(r.endpoints))
	copy(endpoints, r.endpoints)

	sort.Slice(endpoints, func(i, j int) bool {
		if endpoints[i].Path != endpoints[j].Path {
			return endpoints[i].Path < endpoints[j].Path
		}
		return endpoints[i].Method < endpoints[j].Method
	})

	return ServiceDoc{
		Name:        r.name,
		Version:     r.version,
		Description: r.desc,
		BasePath:    r.basePath,
		Endpoints:   endpoints,
	}
}

// Handler returns an HTTP handler that serves the API documentation.
func (r *Registry) Handler() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "public, max-age=300")
		json.NewEncoder(w).Encode(r.Doc())
	}
}

// Count returns the number of registered endpoints.
func (r *Registry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.endpoints)
}

// Tags returns all unique tags across endpoints.
func (r *Registry) Tags() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	seen := make(map[string]bool)
	for _, ep := range r.endpoints {
		for _, tag := range ep.Tags {
			seen[tag] = true
		}
	}

	tags := make([]string, 0, len(seen))
	for tag := range seen {
		tags = append(tags, tag)
	}
	sort.Strings(tags)
	return tags
}
