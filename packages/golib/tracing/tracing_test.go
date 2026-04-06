package tracing

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNewTraceID_Unique(t *testing.T) {
	seen := make(map[string]bool, 100)
	for i := 0; i < 100; i++ {
		id := NewTraceID()
		s := id.String()
		if len(s) != 32 {
			t.Fatalf("TraceID length = %d, want 32", len(s))
		}
		if seen[s] {
			t.Fatalf("duplicate TraceID on iteration %d: %s", i, s)
		}
		seen[s] = true
	}
}

func TestNewSpanID_Unique(t *testing.T) {
	seen := make(map[string]bool, 100)
	for i := 0; i < 100; i++ {
		id := NewSpanID()
		s := id.String()
		if len(s) != 16 {
			t.Fatalf("SpanID length = %d, want 16", len(s))
		}
		if seen[s] {
			t.Fatalf("duplicate SpanID on iteration %d: %s", i, s)
		}
		seen[s] = true
	}
}

func TestParseTraceParent_Valid(t *testing.T) {
	header := "00-4bf92f3577b6a27bd529b1dd4a1e5815-00f067aa0ba902b7-01"
	traceID, spanID, flags, err := ParseTraceParent(header)
	if err != nil {
		t.Fatalf("ParseTraceParent(%q) error: %v", header, err)
	}
	if traceID.String() != "4bf92f3577b6a27bd529b1dd4a1e5815" {
		t.Errorf("traceID = %s, want 4bf92f3577b6a27bd529b1dd4a1e5815", traceID.String())
	}
	if spanID.String() != "00f067aa0ba902b7" {
		t.Errorf("spanID = %s, want 00f067aa0ba902b7", spanID.String())
	}
	if flags != 0x01 {
		t.Errorf("flags = %02x, want 01", flags)
	}
}

func TestParseTraceParent_Invalid(t *testing.T) {
	tests := []struct {
		name   string
		header string
	}{
		{"empty", ""},
		{"too few parts", "00-abc-def"},
		{"too many parts", "00-abc-def-01-extra"},
		{"bad version length", "0-4bf92f3577b6a27bd529b1dd4a1e5815-00f067aa0ba902b7-01"},
		{"bad trace-id hex", "00-zzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzz-00f067aa0ba902b7-01"},
		{"bad trace-id length", "00-4bf92f35-00f067aa0ba902b7-01"},
		{"bad span-id hex", "00-4bf92f3577b6a27bd529b1dd4a1e5815-zzzzzzzzzzzzzzzz-01"},
		{"bad span-id length", "00-4bf92f3577b6a27bd529b1dd4a1e5815-00f067-01"},
		{"bad flags", "00-4bf92f3577b6a27bd529b1dd4a1e5815-00f067aa0ba902b7-zz"},
		{"zero trace-id", "00-00000000000000000000000000000000-00f067aa0ba902b7-01"},
		{"zero span-id", "00-4bf92f3577b6a27bd529b1dd4a1e5815-0000000000000000-01"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, _, err := ParseTraceParent(tt.header)
			if err == nil {
				t.Errorf("ParseTraceParent(%q) expected error, got nil", tt.header)
			}
		})
	}
}

func TestFormatTraceParent(t *testing.T) {
	header := "00-4bf92f3577b6a27bd529b1dd4a1e5815-00f067aa0ba902b7-01"
	traceID, spanID, flags, err := ParseTraceParent(header)
	if err != nil {
		t.Fatalf("ParseTraceParent error: %v", err)
	}
	got := FormatTraceParent(traceID, spanID, flags)
	if got != header {
		t.Errorf("FormatTraceParent = %q, want %q", got, header)
	}
}

func TestParseFormat_Roundtrip(t *testing.T) {
	original := "00-0af7651916cd43dd8448eb211c80319c-b7ad6b7169203331-00"
	traceID, spanID, flags, err := ParseTraceParent(original)
	if err != nil {
		t.Fatalf("ParseTraceParent error: %v", err)
	}
	result := FormatTraceParent(traceID, spanID, flags)
	if result != original {
		t.Errorf("roundtrip = %q, want %q", result, original)
	}
}

func TestStartSpan_NewTrace(t *testing.T) {
	ctx, span := StartSpan(context.Background(), "test-op")
	if span == nil {
		t.Fatal("StartSpan returned nil span")
	}
	if span.TraceID.IsZero() {
		t.Error("StartSpan created zero TraceID")
	}
	if span.SpanID.IsZero() {
		t.Error("StartSpan created zero SpanID")
	}
	if !span.ParentSpanID.IsZero() {
		t.Error("new trace should have zero ParentSpanID")
	}
	if span.Name != "test-op" {
		t.Errorf("span.Name = %q, want %q", span.Name, "test-op")
	}
	if span.StartTime.IsZero() {
		t.Error("span.StartTime is zero")
	}
	if got := SpanFromContext(ctx); got != span {
		t.Error("span not stored in context")
	}
}

func TestStartSpan_InheritTrace(t *testing.T) {
	ctx, parent := StartSpan(context.Background(), "parent")
	_, child := StartSpan(ctx, "child")

	if child.TraceID != parent.TraceID {
		t.Errorf("child TraceID = %s, want %s", child.TraceID, parent.TraceID)
	}
	if child.SpanID == parent.SpanID {
		t.Error("child SpanID should differ from parent SpanID")
	}
	if child.ParentSpanID != parent.SpanID {
		t.Errorf("child ParentSpanID = %s, want %s", child.ParentSpanID, parent.SpanID)
	}
}

func TestStartSpan_Nested(t *testing.T) {
	ctx, root := StartSpan(context.Background(), "root")
	ctx, child1 := StartSpan(ctx, "child1")
	_, child2 := StartSpan(ctx, "child2")

	// All share the same trace.
	if child1.TraceID != root.TraceID {
		t.Error("child1 should share root's TraceID")
	}
	if child2.TraceID != root.TraceID {
		t.Error("child2 should share root's TraceID")
	}

	// child1 is child of root.
	if child1.ParentSpanID != root.SpanID {
		t.Error("child1.ParentSpanID should be root.SpanID")
	}

	// child2 is child of child1.
	if child2.ParentSpanID != child1.SpanID {
		t.Error("child2.ParentSpanID should be child1.SpanID")
	}

	// root has child1 in Children.
	if len(root.Children) != 1 || root.Children[0] != child1 {
		t.Error("root.Children should contain child1")
	}

	// child1 has child2 in Children.
	if len(child1.Children) != 1 || child1.Children[0] != child2 {
		t.Error("child1.Children should contain child2")
	}
}

func TestEndSpan(t *testing.T) {
	_, span := StartSpan(context.Background(), "op")
	if !span.EndTime.IsZero() {
		t.Error("EndTime should be zero before EndSpan")
	}
	EndSpan(span)
	if span.EndTime.IsZero() {
		t.Error("EndTime should be set after EndSpan")
	}
	if span.EndTime.Before(span.StartTime) {
		t.Error("EndTime should not be before StartTime")
	}
}

func TestSpanFromContext_NoSpan(t *testing.T) {
	span := SpanFromContext(context.Background())
	if span != nil {
		t.Error("SpanFromContext on empty context should return nil")
	}
}

func TestTraceIDFromContext(t *testing.T) {
	ctx, span := StartSpan(context.Background(), "op")
	got := TraceIDFromContext(ctx)
	if got != span.TraceID.String() {
		t.Errorf("TraceIDFromContext = %q, want %q", got, span.TraceID.String())
	}

	// Empty context returns empty string.
	if got := TraceIDFromContext(context.Background()); got != "" {
		t.Errorf("TraceIDFromContext(empty) = %q, want empty", got)
	}
}

func TestMiddleware_CreatesSpan(t *testing.T) {
	var captured *Span
	handler := Middleware("test-svc")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		captured = SpanFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if captured == nil {
		t.Fatal("middleware did not create a span")
	}
	if captured.TraceID.IsZero() {
		t.Error("span has zero TraceID")
	}
	if captured.ServiceName != "test-svc" {
		t.Errorf("ServiceName = %q, want %q", captured.ServiceName, "test-svc")
	}

	// Response should contain Traceparent header.
	tp := rec.Header().Get("Traceparent")
	if tp == "" {
		t.Error("response missing Traceparent header")
	}
}

func TestMiddleware_PropagatesTraceParent(t *testing.T) {
	incomingTP := "00-4bf92f3577b6a27bd529b1dd4a1e5815-00f067aa0ba902b7-01"

	var captured *Span
	handler := Middleware("test-svc")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		captured = SpanFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
	req.Header.Set("Traceparent", incomingTP)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if captured == nil {
		t.Fatal("middleware did not create a span")
	}

	// Child span should inherit the trace ID from the incoming traceparent.
	if captured.TraceID.String() != "4bf92f3577b6a27bd529b1dd4a1e5815" {
		t.Errorf("TraceID = %s, want 4bf92f3577b6a27bd529b1dd4a1e5815", captured.TraceID.String())
	}

	// Child span should have a different span ID.
	if captured.SpanID.String() == "00f067aa0ba902b7" {
		t.Error("child span should have a new SpanID, not the parent's")
	}

	// ParentSpanID should be the incoming span ID.
	if captured.ParentSpanID.String() != "00f067aa0ba902b7" {
		t.Errorf("ParentSpanID = %s, want 00f067aa0ba902b7", captured.ParentSpanID.String())
	}
}

func TestMiddleware_SetsAttributes(t *testing.T) {
	var captured *Span
	handler := Middleware("test-svc")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		captured = SpanFromContext(r.Context())
		w.WriteHeader(http.StatusCreated)
	}))

	req := httptest.NewRequest(http.MethodPost, "/api/users", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if captured == nil {
		t.Fatal("middleware did not create a span")
	}
	if captured.Attributes["http.method"] != "POST" {
		t.Errorf("http.method = %q, want POST", captured.Attributes["http.method"])
	}
	if captured.Attributes["http.url"] != "/api/users" {
		t.Errorf("http.url = %q, want /api/users", captured.Attributes["http.url"])
	}
	if captured.Attributes["http.status_code"] != "201" {
		t.Errorf("http.status_code = %q, want 201", captured.Attributes["http.status_code"])
	}
}

func TestMiddleware_RecordsStatusCode(t *testing.T) {
	tests := []struct {
		code   int
		status string
	}{
		{http.StatusOK, "ok"},
		{http.StatusCreated, "ok"},
		{http.StatusBadRequest, "error"},
		{http.StatusInternalServerError, "error"},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("status_%d", tt.code), func(t *testing.T) {
			var captured *Span
			handler := Middleware("svc")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				captured = SpanFromContext(r.Context())
				w.WriteHeader(tt.code)
			}))

			req := httptest.NewRequest(http.MethodGet, "/", nil)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if captured.Attributes["http.status_code"] != fmt.Sprintf("%d", tt.code) {
				t.Errorf("http.status_code = %q, want %d", captured.Attributes["http.status_code"], tt.code)
			}
			if captured.Status != tt.status {
				t.Errorf("Status = %q, want %q", captured.Status, tt.status)
			}
		})
	}
}

func TestInjectHeaders(t *testing.T) {
	ctx, span := StartSpan(context.Background(), "outgoing")
	req := httptest.NewRequest(http.MethodGet, "http://other-service/api", nil)
	InjectHeaders(ctx, req)

	tp := req.Header.Get("Traceparent")
	if tp == "" {
		t.Fatal("InjectHeaders did not set Traceparent")
	}

	// Parse the injected traceparent and verify it matches the span.
	traceID, spanID, flags, err := ParseTraceParent(tp)
	if err != nil {
		t.Fatalf("injected traceparent is invalid: %v", err)
	}
	if traceID != span.TraceID {
		t.Errorf("injected traceID = %s, want %s", traceID, span.TraceID)
	}
	if spanID != span.SpanID {
		t.Errorf("injected spanID = %s, want %s", spanID, span.SpanID)
	}
	if flags != 0x01 {
		t.Errorf("injected flags = %02x, want 01", flags)
	}
}

func TestMiddleware_NoTraceParent(t *testing.T) {
	var captured *Span
	handler := Middleware("test-svc")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		captured = SpanFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/data", nil)
	// No Traceparent header set.
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if captured == nil {
		t.Fatal("middleware did not create a span")
	}
	if captured.TraceID.IsZero() {
		t.Error("middleware should create a new TraceID when no incoming traceparent")
	}
	if captured.SpanID.IsZero() {
		t.Error("middleware should create a new SpanID")
	}
	if !captured.ParentSpanID.IsZero() {
		t.Error("no incoming traceparent means ParentSpanID should be zero")
	}

	// Response should still have Traceparent.
	tp := rec.Header().Get("Traceparent")
	if tp == "" {
		t.Error("response missing Traceparent header even without incoming header")
	}
}
