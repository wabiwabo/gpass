// Package correlation provides structured correlation and causation ID
// propagation for distributed request tracing across service boundaries.
//
// Every inbound request gets a correlation ID. When a service makes
// downstream calls as part of handling a request, the correlation ID
// propagates while a new causation ID links parent→child relationships.
package correlation

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"sync"
	"time"
)

// Header names for propagation.
const (
	HeaderCorrelationID = "X-Correlation-ID"
	HeaderCausationID   = "X-Causation-ID"
	HeaderRequestID     = "X-Request-ID"
	HeaderOriginService = "X-Origin-Service"
)

type contextKey struct{}

// Info holds correlation metadata for a request.
type Info struct {
	CorrelationID string    `json:"correlation_id"`
	CausationID   string    `json:"causation_id,omitempty"`
	RequestID     string    `json:"request_id"`
	OriginService string    `json:"origin_service,omitempty"`
	Timestamp     time.Time `json:"timestamp"`
	Depth         int       `json:"depth"` // hop count
}

// FromContext extracts correlation info from context.
func FromContext(ctx context.Context) (Info, bool) {
	info, ok := ctx.Value(contextKey{}).(Info)
	return info, ok
}

// WithInfo stores correlation info in context.
func WithInfo(ctx context.Context, info Info) context.Context {
	return context.WithValue(ctx, contextKey{}, info)
}

// GenerateID creates a new unique ID (16 hex chars = 8 bytes entropy).
func GenerateID() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		// Fallback: timestamp-based (should never happen with /dev/urandom).
		t := time.Now().UnixNano()
		for i := range b {
			b[i] = byte(t >> (i * 8))
		}
	}
	return hex.EncodeToString(b)
}

// Middleware injects correlation info into the request context.
// If no correlation ID is present, one is generated.
func Middleware(serviceName string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			correlationID := r.Header.Get(HeaderCorrelationID)
			if correlationID == "" {
				correlationID = GenerateID()
			}

			causationID := r.Header.Get(HeaderCausationID)
			requestID := r.Header.Get(HeaderRequestID)
			if requestID == "" {
				requestID = GenerateID()
			}

			origin := r.Header.Get(HeaderOriginService)
			depth := 0
			if causationID != "" {
				depth = 1 // At least one hop if causation present.
			}

			info := Info{
				CorrelationID: correlationID,
				CausationID:   causationID,
				RequestID:     requestID,
				OriginService: origin,
				Timestamp:     time.Now(),
				Depth:         depth,
			}

			ctx := WithInfo(r.Context(), info)

			// Set response headers for traceability.
			w.Header().Set(HeaderCorrelationID, correlationID)
			w.Header().Set(HeaderRequestID, requestID)

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// Propagate returns headers for downstream calls, linking parent→child.
func Propagate(ctx context.Context, serviceName string) http.Header {
	h := http.Header{}
	info, ok := FromContext(ctx)
	if !ok {
		// No correlation context — start a new chain.
		id := GenerateID()
		h.Set(HeaderCorrelationID, id)
		h.Set(HeaderRequestID, GenerateID())
		h.Set(HeaderOriginService, serviceName)
		return h
	}

	h.Set(HeaderCorrelationID, info.CorrelationID)
	h.Set(HeaderCausationID, info.RequestID) // Current request becomes the cause.
	h.Set(HeaderRequestID, GenerateID())     // New request ID for downstream.
	h.Set(HeaderOriginService, serviceName)
	return h
}

// RoundTripper wraps http.RoundTripper to automatically propagate correlation headers.
type RoundTripper struct {
	Base        http.RoundTripper
	ServiceName string
}

// RoundTrip implements http.RoundTripper.
func (rt *RoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	base := rt.Base
	if base == nil {
		base = http.DefaultTransport
	}

	headers := Propagate(req.Context(), rt.ServiceName)
	for k, v := range headers {
		if len(v) > 0 {
			req.Header.Set(k, v[0])
		}
	}

	return base.RoundTrip(req)
}

// Chain tracks the full correlation chain for debugging.
type Chain struct {
	mu    sync.Mutex
	hops  []HopInfo
	maxID int
}

// HopInfo records a single hop in the correlation chain.
type HopInfo struct {
	ID        int       `json:"id"`
	Service   string    `json:"service"`
	RequestID string    `json:"request_id"`
	Timestamp time.Time `json:"timestamp"`
	Duration  time.Duration `json:"duration,omitempty"`
}

// NewChain creates a new correlation chain tracker.
func NewChain() *Chain {
	return &Chain{}
}

// AddHop records a hop in the chain.
func (c *Chain) AddHop(service, requestID string) int {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.maxID++
	c.hops = append(c.hops, HopInfo{
		ID:        c.maxID,
		Service:   service,
		RequestID: requestID,
		Timestamp: time.Now(),
	})
	return c.maxID
}

// Hops returns all recorded hops.
func (c *Chain) Hops() []HopInfo {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]HopInfo, len(c.hops))
	copy(out, c.hops)
	return out
}

// Len returns the number of hops.
func (c *Chain) Len() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.hops)
}
