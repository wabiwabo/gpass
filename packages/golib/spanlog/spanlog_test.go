package spanlog

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"
)

func TestWithFields_FromContext(t *testing.T) {
	ctx := WithFields(context.Background(), Fields{
		TraceID:   "trace-123",
		SpanID:    "span-456",
		Service:   "identity",
		RequestID: "req-789",
	})

	f := FromContext(ctx)
	if f.TraceID != "trace-123" {
		t.Errorf("TraceID = %q", f.TraceID)
	}
	if f.Service != "identity" {
		t.Errorf("Service = %q", f.Service)
	}
}

func TestFromContext_Empty(t *testing.T) {
	f := FromContext(context.Background())
	if f.TraceID != "" || f.Service != "" {
		t.Error("should be empty")
	}
}

func TestLogger(t *testing.T) {
	var buf bytes.Buffer
	base := slog.New(slog.NewJSONHandler(&buf, nil))

	ctx := WithFields(context.Background(), Fields{
		TraceID: "t-1",
		SpanID:  "s-1",
		Service: "svc",
	})

	l := Logger(ctx, base)
	l.Info("test message")

	output := buf.String()
	if !strings.Contains(output, "t-1") {
		t.Error("should contain trace_id")
	}
	if !strings.Contains(output, "s-1") {
		t.Error("should contain span_id")
	}
	if !strings.Contains(output, "svc") {
		t.Error("should contain service")
	}
}

func TestLogger_EmptyContext(t *testing.T) {
	var buf bytes.Buffer
	base := slog.New(slog.NewJSONHandler(&buf, nil))

	l := Logger(context.Background(), base)
	l.Info("no trace")

	output := buf.String()
	if strings.Contains(output, "trace_id") {
		t.Error("should not have trace_id when not set")
	}
}

func TestLogger_NilBase(t *testing.T) {
	ctx := WithFields(context.Background(), Fields{TraceID: "t"})
	l := Logger(ctx, nil)
	// Should not panic, uses slog.Default()
	_ = l
}

func TestInfo(t *testing.T) {
	ctx := WithFields(context.Background(), Fields{TraceID: "t"})
	// Should not panic
	Info(ctx, "test info")
}

func TestWarn(t *testing.T) {
	ctx := context.Background()
	Warn(ctx, "test warn")
}

func TestError(t *testing.T) {
	ctx := context.Background()
	Error(ctx, "test error")
}

func TestDebug(t *testing.T) {
	ctx := context.Background()
	Debug(ctx, "test debug")
}

func TestPartialFields(t *testing.T) {
	var buf bytes.Buffer
	base := slog.New(slog.NewJSONHandler(&buf, nil))

	ctx := WithFields(context.Background(), Fields{TraceID: "only-trace"})
	Logger(ctx, base).Info("partial")

	output := buf.String()
	if !strings.Contains(output, "only-trace") {
		t.Error("should contain trace_id")
	}
	if strings.Contains(output, "span_id") {
		t.Error("should not have span_id when not set")
	}
}
