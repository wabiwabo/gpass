package httputil

import (
	"net/http"
	"strings"
	"sync"
)

// VersionedRouter routes requests to version-specific handlers based on
// the Accept header. It supports content negotiation via
// "application/vnd.garudapass.v1+json" style media types.
type VersionedRouter struct {
	versions map[string]*http.ServeMux
	default_ string
	mu       sync.RWMutex
}

// NewVersionedRouter creates a new versioned router with the given default version.
func NewVersionedRouter(defaultVersion string) *VersionedRouter {
	return &VersionedRouter{
		versions: make(map[string]*http.ServeMux),
		default_: defaultVersion,
	}
}

// Version returns the ServeMux for the given API version, creating it if needed.
func (r *VersionedRouter) Version(version string) *http.ServeMux {
	r.mu.Lock()
	defer r.mu.Unlock()
	if mux, ok := r.versions[version]; ok {
		return mux
	}
	mux := http.NewServeMux()
	r.versions[version] = mux
	return mux
}

// ServeHTTP routes based on Accept header version or falls back to the default version.
// It parses version from Accept headers like "application/vnd.garudapass.v1+json".
func (r *VersionedRouter) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	version := r.extractVersion(req)

	r.mu.RLock()
	defer r.mu.RUnlock()

	if mux, ok := r.versions[version]; ok {
		mux.ServeHTTP(w, req)
		return
	}

	// If the requested version is not the default, return 406.
	if version != r.default_ {
		http.Error(w, "unsupported API version: "+version, http.StatusNotAcceptable)
		return
	}

	// Default version also not registered — 406.
	http.Error(w, "unsupported API version: "+version, http.StatusNotAcceptable)
}

// extractVersion parses the API version from the Accept header.
// Expected format: "application/vnd.garudapass.v1+json"
// Falls back to the default version if no version is specified.
func (r *VersionedRouter) extractVersion(req *http.Request) string {
	accept := req.Header.Get("Accept")
	if accept == "" {
		return r.default_
	}

	// Look for "application/vnd.garudapass.VERSION+json"
	const prefix = "application/vnd.garudapass."
	for _, part := range strings.Split(accept, ",") {
		part = strings.TrimSpace(part)
		if strings.HasPrefix(part, prefix) {
			// Extract version between prefix and "+json"
			rest := strings.TrimPrefix(part, prefix)
			if idx := strings.Index(rest, "+"); idx > 0 {
				return rest[:idx]
			}
			return rest
		}
	}

	return r.default_
}
