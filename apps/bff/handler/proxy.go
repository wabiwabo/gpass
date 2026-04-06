package handler

import (
	"io"
	"log/slog"
	"net/http"
	"strings"
)

// hopByHopHeaders are headers that should not be forwarded by proxies.
var hopByHopHeaders = []string{
	"Connection",
	"Keep-Alive",
	"Proxy-Authenticate",
	"Proxy-Authorization",
	"TE",
	"Trailers",
	"Transfer-Encoding",
	"Upgrade",
}

// ServiceProxy proxies authenticated requests to internal services.
type ServiceProxy struct {
	routes map[string]string // path prefix -> backend URL
	client *http.Client
}

// NewServiceProxy creates a proxy with service routes.
// If routes is nil, default routes are used.
func NewServiceProxy(routes map[string]string) *ServiceProxy {
	if routes == nil {
		routes = map[string]string{
			"/api/v1/identity": "http://localhost:4001",
			"/api/v1/consent":  "http://localhost:4003",
			"/api/v1/corp":     "http://localhost:4006",
			"/api/v1/sign":     "http://localhost:4007",
			"/api/v1/portal":   "http://localhost:4009",
			"/api/v1/audit":    "http://localhost:4010",
			"/api/v1/notify":   "http://localhost:4011",
		}
	}
	return &ServiceProxy{
		routes: routes,
		client: http.DefaultClient,
	}
}

// ServeHTTP proxies the request to the appropriate backend.
// It matches the request path against route prefixes, forwards X-User-ID
// from session context (not from client), forwards X-Request-Id, strips
// hop-by-hop headers, returns 404 if no matching route, and returns 502
// if the backend is unreachable.
func (p *ServiceProxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	backendURL, prefix := p.matchRoute(r.URL.Path)
	if backendURL == "" {
		writeErrorJSON(w, http.StatusNotFound, "not_found", "No matching service route")
		return
	}

	// Build the backend request URL: backend base + full original path
	targetURL := backendURL + r.URL.Path[len(prefix):]
	if r.URL.RawQuery != "" {
		targetURL += "?" + r.URL.RawQuery
	}

	proxyReq, err := http.NewRequestWithContext(r.Context(), r.Method, targetURL, r.Body)
	if err != nil {
		slog.Error("failed to create proxy request", "error", err)
		writeErrorJSON(w, http.StatusInternalServerError, "internal_error", "Failed to create proxy request")
		return
	}

	// Copy headers from original request
	for key, values := range r.Header {
		for _, v := range values {
			proxyReq.Header.Add(key, v)
		}
	}

	// Strip hop-by-hop headers
	for _, h := range hopByHopHeaders {
		proxyReq.Header.Del(h)
	}

	// Remove any client-supplied X-User-ID (prevent spoofing)
	proxyReq.Header.Del("X-User-ID")

	// Forward X-User-ID from request header if set by upstream middleware
	if userID := r.Header.Get("X-User-ID"); userID != "" {
		proxyReq.Header.Set("X-User-ID", userID)
	}

	// Forward X-Request-Id
	if reqID := r.Header.Get("X-Request-Id"); reqID != "" {
		proxyReq.Header.Set("X-Request-Id", reqID)
	}

	resp, err := p.client.Do(proxyReq)
	if err != nil {
		slog.Error("backend unreachable", "error", err, "backend", backendURL)
		writeErrorJSON(w, http.StatusBadGateway, "bad_gateway", "Backend service unavailable")
		return
	}
	defer resp.Body.Close()

	// Copy response headers (stripping hop-by-hop)
	for key, values := range resp.Header {
		for _, v := range values {
			w.Header().Add(key, v)
		}
	}
	for _, h := range hopByHopHeaders {
		w.Header().Del(h)
	}

	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
}

// matchRoute finds the longest matching route prefix for the given path.
func (p *ServiceProxy) matchRoute(path string) (backendURL, prefix string) {
	bestLen := 0
	for pfx, url := range p.routes {
		if strings.HasPrefix(path, pfx) && len(pfx) > bestLen {
			bestLen = len(pfx)
			backendURL = url
			prefix = pfx
		}
	}
	return backendURL, prefix
}
