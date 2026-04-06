package tracing

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// TraceID is a 16-byte unique identifier for a distributed trace.
type TraceID [16]byte

// SpanID is an 8-byte unique identifier for a span within a trace.
type SpanID [8]byte

// String returns the hex-encoded representation of the TraceID (32 chars).
func (t TraceID) String() string {
	return hex.EncodeToString(t[:])
}

// IsZero reports whether the TraceID is the zero value.
func (t TraceID) IsZero() bool {
	return t == TraceID{}
}

// String returns the hex-encoded representation of the SpanID (16 chars).
func (s SpanID) String() string {
	return hex.EncodeToString(s[:])
}

// IsZero reports whether the SpanID is the zero value.
func (s SpanID) IsZero() bool {
	return s == SpanID{}
}

// Span represents a unit of work within a distributed trace.
type Span struct {
	TraceID      TraceID
	SpanID       SpanID
	ParentSpanID SpanID
	Name         string
	ServiceName  string
	StartTime    time.Time
	EndTime      time.Time
	Status       string
	Attributes   map[string]string
	Children     []*Span
}

// contextKey is an unexported type for context keys in this package.
type contextKey struct{}

// NewTraceID generates a new random TraceID using crypto/rand.
func NewTraceID() TraceID {
	var id TraceID
	_, _ = rand.Read(id[:])
	return id
}

// NewSpanID generates a new random SpanID using crypto/rand.
func NewSpanID() SpanID {
	var id SpanID
	_, _ = rand.Read(id[:])
	return id
}

// ParseTraceParent parses a W3C traceparent header string.
// Format: version-traceid-spanid-flags (e.g., "00-4bf92f3577b6a27bd529b1dd4a1e5815-00f067aa0ba902b7-01").
func ParseTraceParent(header string) (traceID TraceID, spanID SpanID, flags byte, err error) {
	parts := strings.Split(header, "-")
	if len(parts) != 4 {
		return traceID, spanID, 0, fmt.Errorf("traceparent: expected 4 parts, got %d", len(parts))
	}

	if len(parts[0]) != 2 {
		return traceID, spanID, 0, fmt.Errorf("traceparent: invalid version length %d", len(parts[0]))
	}

	traceBytes, err := hex.DecodeString(parts[1])
	if err != nil || len(traceBytes) != 16 {
		return traceID, spanID, 0, fmt.Errorf("traceparent: invalid trace-id %q", parts[1])
	}
	copy(traceID[:], traceBytes)

	spanBytes, err := hex.DecodeString(parts[2])
	if err != nil || len(spanBytes) != 8 {
		return traceID, spanID, 0, fmt.Errorf("traceparent: invalid span-id %q", parts[2])
	}
	copy(spanID[:], spanBytes)

	flagBytes, err := hex.DecodeString(parts[3])
	if err != nil || len(flagBytes) != 1 {
		return traceID, spanID, 0, fmt.Errorf("traceparent: invalid flags %q", parts[3])
	}
	flags = flagBytes[0]

	if traceID.IsZero() {
		return traceID, spanID, 0, fmt.Errorf("traceparent: trace-id is all zeros")
	}
	if spanID.IsZero() {
		return traceID, spanID, 0, fmt.Errorf("traceparent: span-id is all zeros")
	}

	return traceID, spanID, flags, nil
}

// FormatTraceParent formats a W3C traceparent header string.
func FormatTraceParent(traceID TraceID, spanID SpanID, flags byte) string {
	return fmt.Sprintf("00-%s-%s-%02x", traceID.String(), spanID.String(), flags)
}

// StartSpan creates a new span and stores it in the returned context.
// If a parent span exists in the context, the new span inherits its TraceID
// and records the parent's SpanID. Otherwise, a new trace is started.
func StartSpan(ctx context.Context, name string) (context.Context, *Span) {
	span := &Span{
		Name:       name,
		SpanID:     NewSpanID(),
		StartTime:  time.Now(),
		Status:     "ok",
		Attributes: make(map[string]string),
	}

	if parent := SpanFromContext(ctx); parent != nil {
		span.TraceID = parent.TraceID
		span.ParentSpanID = parent.SpanID
		parent.Children = append(parent.Children, span)
	} else {
		span.TraceID = NewTraceID()
	}

	return context.WithValue(ctx, contextKey{}, span), span
}

// EndSpan marks the span as complete by setting its EndTime.
func EndSpan(span *Span) {
	if span != nil {
		span.EndTime = time.Now()
	}
}

// SpanFromContext retrieves the current span from the context.
// Returns nil if no span is stored in the context.
func SpanFromContext(ctx context.Context) *Span {
	span, _ := ctx.Value(contextKey{}).(*Span)
	return span
}

// TraceIDFromContext returns the trace ID string from the current span in the context.
// Returns an empty string if no span is stored in the context.
func TraceIDFromContext(ctx context.Context) string {
	if span := SpanFromContext(ctx); span != nil {
		return span.TraceID.String()
	}
	return ""
}

// responseWriter wraps http.ResponseWriter to capture the status code.
type responseWriter struct {
	http.ResponseWriter
	statusCode int
	written    bool
}

// WriteHeader captures the status code and delegates to the underlying ResponseWriter.
func (rw *responseWriter) WriteHeader(code int) {
	if !rw.written {
		rw.statusCode = code
		rw.written = true
	}
	rw.ResponseWriter.WriteHeader(code)
}

// Write delegates to the underlying ResponseWriter, defaulting status to 200
// if WriteHeader was not called.
func (rw *responseWriter) Write(b []byte) (int, error) {
	if !rw.written {
		rw.statusCode = http.StatusOK
		rw.written = true
	}
	return rw.ResponseWriter.Write(b)
}

// Middleware returns HTTP middleware that creates spans for incoming requests.
// It extracts the traceparent header, creates a child span (or new trace),
// sets span attributes (http.method, http.url, http.status_code),
// propagates the traceparent in the response, and records duration and status.
func Middleware(serviceName string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()

			// Extract incoming traceparent if present.
			var parentTraceID TraceID
			var parentSpanID SpanID
			var flags byte
			if tp := r.Header.Get("Traceparent"); tp != "" {
				var err error
				parentTraceID, parentSpanID, flags, err = ParseTraceParent(tp)
				if err == nil {
					// Create a synthetic parent span in context so StartSpan inherits the trace.
					parentSpan := &Span{
						TraceID: parentTraceID,
						SpanID:  parentSpanID,
					}
					ctx = context.WithValue(ctx, contextKey{}, parentSpan)
				}
			}

			ctx, span := StartSpan(ctx, r.URL.Path)
			span.ServiceName = serviceName
			span.Attributes["http.method"] = r.Method
			span.Attributes["http.url"] = r.URL.String()

			// Propagate traceparent in response.
			outFlags := flags
			w.Header().Set("Traceparent", FormatTraceParent(span.TraceID, span.SpanID, outFlags))

			rw := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}
			next.ServeHTTP(rw, r.WithContext(ctx))

			span.Attributes["http.status_code"] = fmt.Sprintf("%d", rw.statusCode)
			if rw.statusCode >= 400 {
				span.Status = "error"
			}
			EndSpan(span)
		})
	}
}

// InjectHeaders injects the traceparent header into an outgoing HTTP request
// from the current span in the context.
func InjectHeaders(ctx context.Context, req *http.Request) {
	if span := SpanFromContext(ctx); span != nil {
		req.Header.Set("Traceparent", FormatTraceParent(span.TraceID, span.SpanID, 0x01))
	}
}
