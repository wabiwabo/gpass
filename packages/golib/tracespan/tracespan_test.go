package tracespan

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestStart(t *testing.T) {
	s := Start("identity", "verify_nik")
	if s.TraceID == "" {
		t.Error("TraceID should not be empty")
	}
	if s.SpanID == "" {
		t.Error("SpanID should not be empty")
	}
	if s.ParentID != "" {
		t.Error("root span should have no parent")
	}
	if s.Operation != "verify_nik" {
		t.Errorf("Operation = %q", s.Operation)
	}
	if s.Service != "identity" {
		t.Errorf("Service = %q", s.Service)
	}
	if s.Status != StatusOK {
		t.Errorf("Status = %q", s.Status)
	}
	if !s.IsRoot() {
		t.Error("should be root")
	}
}

func TestStartChild(t *testing.T) {
	parent := Start("identity", "handle_request")
	child := StartChild(parent, "db_query")

	if child.TraceID != parent.TraceID {
		t.Error("child should inherit trace ID")
	}
	if child.ParentID != parent.SpanID {
		t.Error("child parent should be parent span")
	}
	if child.SpanID == parent.SpanID {
		t.Error("child should have its own span ID")
	}
	if child.IsRoot() {
		t.Error("child should not be root")
	}
}

func TestEnd(t *testing.T) {
	s := Start("svc", "op")
	time.Sleep(1 * time.Millisecond)
	s.End()

	if s.EndTime.IsZero() {
		t.Error("EndTime should be set")
	}
	if s.Duration <= 0 {
		t.Error("Duration should be positive")
	}
}

func TestSetError(t *testing.T) {
	s := Start("svc", "op")
	s.SetError(errors.New("connection refused"))

	if s.Status != StatusError {
		t.Errorf("Status = %q, want error", s.Status)
	}
	if s.Tags["error"] != "connection refused" {
		t.Errorf("error tag = %q", s.Tags["error"])
	}
}

func TestSetTag(t *testing.T) {
	s := Start("svc", "op")
	s.SetTag("http.method", "POST")
	s.SetTag("http.path", "/api/verify")

	if s.Tags["http.method"] != "POST" {
		t.Error("tag not set")
	}
	if s.Tags["http.path"] != "/api/verify" {
		t.Error("tag not set")
	}
}

func TestWithSpan_FromContext(t *testing.T) {
	s := Start("svc", "op")
	ctx := WithSpan(context.Background(), s)

	got := FromContext(ctx)
	if got != s {
		t.Error("should retrieve same span")
	}
}

func TestFromContext_Empty(t *testing.T) {
	got := FromContext(context.Background())
	if got != nil {
		t.Error("should be nil for empty context")
	}
}

func TestStartFromContext_WithParent(t *testing.T) {
	parent := Start("identity", "handle")
	ctx := WithSpan(context.Background(), parent)

	child, ctx2 := StartFromContext(ctx, "identity", "db_query")
	if child.TraceID != parent.TraceID {
		t.Error("should inherit trace ID")
	}
	if child.ParentID != parent.SpanID {
		t.Error("should have parent")
	}

	// New span should be in context
	got := FromContext(ctx2)
	if got != child {
		t.Error("context should have child span")
	}
}

func TestStartFromContext_NoParent(t *testing.T) {
	span, _ := StartFromContext(context.Background(), "svc", "op")
	if !span.IsRoot() {
		t.Error("should be root when no parent in context")
	}
}

func TestTraceID_Length(t *testing.T) {
	s := Start("svc", "op")
	// 16 bytes → 32 hex chars
	if len(s.TraceID) != 32 {
		t.Errorf("TraceID len = %d, want 32", len(s.TraceID))
	}
	// 8 bytes → 16 hex chars
	if len(s.SpanID) != 16 {
		t.Errorf("SpanID len = %d, want 16", len(s.SpanID))
	}
}

func TestUnique_IDs(t *testing.T) {
	traces := make(map[string]bool)
	spans := make(map[string]bool)
	for i := 0; i < 100; i++ {
		s := Start("svc", "op")
		if traces[s.TraceID] {
			t.Fatal("duplicate trace ID")
		}
		if spans[s.SpanID] {
			t.Fatal("duplicate span ID")
		}
		traces[s.TraceID] = true
		spans[s.SpanID] = true
	}
}
