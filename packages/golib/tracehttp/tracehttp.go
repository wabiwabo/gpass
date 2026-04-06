// Package tracehttp provides automatic trace header injection for
// outgoing HTTP calls. Wraps http.RoundTripper to propagate W3C
// Trace Context headers (traceparent, tracestate) and custom
// service identity headers across service boundaries.
package tracehttp

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
)

const (
	// HeaderTraceparent is the W3C Trace Context header.
	HeaderTraceparent = "Traceparent"
	// HeaderTracestate is the W3C trace state header.
	HeaderTracestate = "Tracestate"
)

// Config controls trace injection behavior.
type Config struct {
	ServiceName string // Identifying service name.
	// GenerateIfMissing creates trace IDs if not present in context.
	GenerateIfMissing bool
}

// Transport wraps http.RoundTripper with trace header injection.
type Transport struct {
	Base   http.RoundTripper
	Config Config
}

// NewTransport creates a trace-injecting transport.
func NewTransport(base http.RoundTripper, cfg Config) *Transport {
	if base == nil {
		base = http.DefaultTransport
	}
	return &Transport{Base: base, Config: cfg}
}

// RoundTrip implements http.RoundTripper with trace injection.
func (t *Transport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Propagate existing traceparent if present.
	if tp := req.Header.Get(HeaderTraceparent); tp == "" && t.Config.GenerateIfMissing {
		// Generate new trace context.
		traceID := generateHex(16)
		spanID := generateHex(8)
		req.Header.Set(HeaderTraceparent, fmt.Sprintf("00-%s-%s-01", traceID, spanID))
	}

	// Add service identity.
	if t.Config.ServiceName != "" {
		req.Header.Set("X-Origin-Service", t.Config.ServiceName)
	}

	return t.Base.RoundTrip(req)
}

// Client returns an http.Client using this transport.
func (t *Transport) Client() *http.Client {
	return &http.Client{Transport: t}
}

// Middleware returns HTTP middleware that ensures trace headers
// are present on incoming requests and propagated in responses.
func Middleware(serviceName string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			traceparent := r.Header.Get(HeaderTraceparent)

			if traceparent == "" {
				traceID := generateHex(16)
				spanID := generateHex(8)
				traceparent = fmt.Sprintf("00-%s-%s-01", traceID, spanID)
				r.Header.Set(HeaderTraceparent, traceparent)
			}

			// Echo in response for end-to-end tracing.
			w.Header().Set(HeaderTraceparent, traceparent)
			if ts := r.Header.Get(HeaderTracestate); ts != "" {
				w.Header().Set(HeaderTracestate, ts)
			}
			w.Header().Set("X-Origin-Service", serviceName)

			next.ServeHTTP(w, r)
		})
	}
}

func generateHex(byteLen int) string {
	b := make([]byte, byteLen)
	rand.Read(b)
	return hex.EncodeToString(b)
}
