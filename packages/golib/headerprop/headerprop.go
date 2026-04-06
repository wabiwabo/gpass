// Package headerprop provides standardized header propagation across
// service boundaries. It extracts configured headers from incoming
// requests and injects them into outgoing requests for distributed
// tracing, tenant isolation, and context propagation.
package headerprop

import (
	"net/http"
)

// DefaultHeaders are the standard headers propagated across services.
var DefaultHeaders = []string{
	"X-Request-ID",
	"X-Correlation-ID",
	"X-Causation-ID",
	"X-Tenant-ID",
	"X-User-ID",
	"Traceparent",
	"Tracestate",
	"X-Origin-Service",
}

// Propagator manages header propagation.
type Propagator struct {
	headers []string
}

// New creates a propagator with the given headers.
// If no headers specified, uses DefaultHeaders.
func New(headers ...string) *Propagator {
	if len(headers) == 0 {
		headers = DefaultHeaders
	}
	return &Propagator{headers: headers}
}

// Extract reads propagated headers from an incoming request.
func (p *Propagator) Extract(r *http.Request) map[string]string {
	result := make(map[string]string, len(p.headers))
	for _, h := range p.headers {
		if v := r.Header.Get(h); v != "" {
			result[h] = v
		}
	}
	return result
}

// Inject sets propagated headers on an outgoing request.
func (p *Propagator) Inject(req *http.Request, headers map[string]string) {
	for k, v := range headers {
		req.Header.Set(k, v)
	}
}

// InjectFromRequest copies propagated headers from src to dst.
func (p *Propagator) InjectFromRequest(dst, src *http.Request) {
	for _, h := range p.headers {
		if v := src.Header.Get(h); v != "" {
			dst.Header.Set(h, v)
		}
	}
}

// Middleware extracts propagated headers and stores them in response
// headers for end-to-end traceability.
func (p *Propagator) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Echo propagated headers in response.
		for _, h := range p.headers {
			if v := r.Header.Get(h); v != "" {
				w.Header().Set(h, v)
			}
		}
		next.ServeHTTP(w, r)
	})
}

// RoundTripper wraps http.RoundTripper to propagate headers from context.
type RoundTripper struct {
	Base       http.RoundTripper
	Propagator *Propagator
	Source     *http.Request // Source request to propagate from.
}

// RoundTrip implements http.RoundTripper.
func (rt *RoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	base := rt.Base
	if base == nil {
		base = http.DefaultTransport
	}
	if rt.Source != nil {
		rt.Propagator.InjectFromRequest(req, rt.Source)
	}
	return base.RoundTrip(req)
}

// Headers returns the list of propagated headers.
func (p *Propagator) Headers() []string {
	out := make([]string, len(p.headers))
	copy(out, p.headers)
	return out
}

// Count returns the number of configured headers.
func (p *Propagator) Count() int {
	return len(p.headers)
}
