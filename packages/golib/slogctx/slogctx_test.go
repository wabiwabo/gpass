package slogctx

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"
)

func newBufLogger() (*slog.Logger, *bytes.Buffer) {
	var buf bytes.Buffer
	h := slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	return slog.New(h), &buf
}

func TestWithAndFrom(t *testing.T) {
	logger, _ := newBufLogger()
	ctx := With(context.Background(), logger)
	got := From(ctx)
	if got != logger {
		t.Error("From should return the stored logger")
	}
}

func TestFromDefault(t *testing.T) {
	ctx := context.Background()
	got := From(ctx)
	if got == nil {
		t.Error("From should return non-nil default logger")
	}
}

func TestWithAttrs(t *testing.T) {
	logger, buf := newBufLogger()
	ctx := With(context.Background(), logger)
	ctx = WithAttrs(ctx, slog.String("key", "value"))

	Info(ctx, "test message")
	output := buf.String()
	if !strings.Contains(output, "key") || !strings.Contains(output, "value") {
		t.Errorf("output should contain attr: %s", output)
	}
}

func TestWithRequestID(t *testing.T) {
	logger, buf := newBufLogger()
	ctx := With(context.Background(), logger)
	ctx = WithRequestID(ctx, "req-123")

	Info(ctx, "test")
	if !strings.Contains(buf.String(), "req-123") {
		t.Error("output should contain request_id")
	}
}

func TestWithUserID(t *testing.T) {
	logger, buf := newBufLogger()
	ctx := With(context.Background(), logger)
	ctx = WithUserID(ctx, "user-456")

	Info(ctx, "test")
	if !strings.Contains(buf.String(), "user-456") {
		t.Error("output should contain user_id")
	}
}

func TestWithService(t *testing.T) {
	logger, buf := newBufLogger()
	ctx := With(context.Background(), logger)
	ctx = WithService(ctx, "identity")

	Info(ctx, "test")
	if !strings.Contains(buf.String(), "identity") {
		t.Error("output should contain service")
	}
}

func TestInfo(t *testing.T) {
	logger, buf := newBufLogger()
	ctx := With(context.Background(), logger)
	Info(ctx, "info message", slog.Int("count", 42))
	if !strings.Contains(buf.String(), "info message") {
		t.Error("output should contain message")
	}
	if !strings.Contains(buf.String(), "INFO") {
		t.Error("output should contain INFO level")
	}
}

func TestError(t *testing.T) {
	logger, buf := newBufLogger()
	ctx := With(context.Background(), logger)
	Error(ctx, "error occurred", slog.String("err", "something broke"))
	if !strings.Contains(buf.String(), "ERROR") {
		t.Error("output should contain ERROR level")
	}
}

func TestWarn(t *testing.T) {
	logger, buf := newBufLogger()
	ctx := With(context.Background(), logger)
	Warn(ctx, "warning")
	if !strings.Contains(buf.String(), "WARN") {
		t.Error("output should contain WARN level")
	}
}

func TestDebug(t *testing.T) {
	logger, buf := newBufLogger()
	ctx := With(context.Background(), logger)
	Debug(ctx, "debug info")
	if !strings.Contains(buf.String(), "DEBUG") {
		t.Error("output should contain DEBUG level")
	}
}

func TestMultipleAttrs(t *testing.T) {
	logger, buf := newBufLogger()
	ctx := With(context.Background(), logger)
	ctx = WithRequestID(ctx, "req-1")
	ctx = WithUserID(ctx, "user-2")
	ctx = WithService(ctx, "bff")

	Info(ctx, "multi attrs")
	output := buf.String()
	if !strings.Contains(output, "req-1") {
		t.Error("should contain request_id")
	}
	if !strings.Contains(output, "user-2") {
		t.Error("should contain user_id")
	}
	if !strings.Contains(output, "bff") {
		t.Error("should contain service")
	}
}

func TestFromNilValue(t *testing.T) {
	ctx := context.WithValue(context.Background(), loggerKey, (*slog.Logger)(nil))
	got := From(ctx)
	if got == nil {
		t.Error("should return default when stored nil")
	}
}
