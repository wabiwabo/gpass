// Package tracespan provides lightweight distributed tracing spans
// for tracking request flow across services. Generates trace/span IDs,
// manages parent-child relationships, and records timing.
package tracespan

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"sync"
	"time"
)

type contextKey int

const keySpan contextKey = iota

// Span represents a single unit of work in a distributed trace.
type Span struct {
	TraceID   string            `json:"trace_id"`
	SpanID    string            `json:"span_id"`
	ParentID  string            `json:"parent_id,omitempty"`
	Operation string            `json:"operation"`
	Service   string            `json:"service"`
	StartTime time.Time         `json:"start_time"`
	EndTime   time.Time         `json:"end_time,omitempty"`
	Duration  time.Duration     `json:"duration,omitempty"`
	Status    SpanStatus        `json:"status"`
	Tags      map[string]string `json:"tags,omitempty"`
	mu        sync.Mutex
}

// SpanStatus represents the outcome of a span.
type SpanStatus string

const (
	StatusOK    SpanStatus = "ok"
	StatusError SpanStatus = "error"
)

// Start creates a new root span.
func Start(service, operation string) *Span {
	return &Span{
		TraceID:   generateID(16),
		SpanID:    generateID(8),
		Operation: operation,
		Service:   service,
		StartTime: time.Now(),
		Status:    StatusOK,
	}
}

// StartChild creates a child span from a parent.
func StartChild(parent *Span, operation string) *Span {
	return &Span{
		TraceID:   parent.TraceID,
		SpanID:    generateID(8),
		ParentID:  parent.SpanID,
		Operation: operation,
		Service:   parent.Service,
		StartTime: time.Now(),
		Status:    StatusOK,
	}
}

// End completes the span and records duration.
func (s *Span) End() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.EndTime = time.Now()
	s.Duration = s.EndTime.Sub(s.StartTime)
}

// SetError marks the span as errored.
func (s *Span) SetError(err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Status = StatusError
	if s.Tags == nil {
		s.Tags = make(map[string]string)
	}
	s.Tags["error"] = err.Error()
}

// SetTag adds a tag to the span.
func (s *Span) SetTag(key, value string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.Tags == nil {
		s.Tags = make(map[string]string)
	}
	s.Tags[key] = value
}

// IsRoot returns true if this is a root span (no parent).
func (s *Span) IsRoot() bool {
	return s.ParentID == ""
}

// WithSpan stores a span in context.
func WithSpan(ctx context.Context, span *Span) context.Context {
	return context.WithValue(ctx, keySpan, span)
}

// FromContext extracts the current span from context.
func FromContext(ctx context.Context) *Span {
	s, _ := ctx.Value(keySpan).(*Span)
	return s
}

// StartFromContext creates a child span from the context's current span.
// If no span exists, creates a new root span.
func StartFromContext(ctx context.Context, service, operation string) (*Span, context.Context) {
	parent := FromContext(ctx)
	var span *Span
	if parent != nil {
		span = StartChild(parent, operation)
		span.Service = service
	} else {
		span = Start(service, operation)
	}
	return span, WithSpan(ctx, span)
}

func generateID(bytes int) string {
	b := make([]byte, bytes)
	if _, err := io.ReadFull(rand.Reader, b); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}
