package logging

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"strings"
	"testing"
)

func TestSetup_SetsGlobalLogger(t *testing.T) {
	before := slog.Default()
	cleanup := Setup(Config{
		ServiceName: "test-service",
		Environment: "testing",
		Level:       "debug",
		Format:      "text",
	})
	defer cleanup()

	after := slog.Default()
	if before == after {
		t.Error("expected Setup to replace the global logger")
	}
}

func TestParseLevel(t *testing.T) {
	tests := []struct {
		input    string
		expected slog.Level
	}{
		{"debug", slog.LevelDebug},
		{"DEBUG", slog.LevelDebug},
		{"info", slog.LevelInfo},
		{"INFO", slog.LevelInfo},
		{"warn", slog.LevelWarn},
		{"WARN", slog.LevelWarn},
		{"warning", slog.LevelWarn},
		{"error", slog.LevelError},
		{"ERROR", slog.LevelError},
		{"unknown", slog.LevelInfo},
		{"", slog.LevelInfo},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := ParseLevel(tt.input)
			if got != tt.expected {
				t.Errorf("ParseLevel(%q) = %v, want %v", tt.input, got, tt.expected)
			}
		})
	}
}

func TestFromContext_IncludesRequestID(t *testing.T) {
	// Capture log output.
	var buf bytes.Buffer
	handler := slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	slog.SetDefault(slog.New(handler))

	ctx := WithRequestID(context.Background(), "req-123")
	logger := FromContext(ctx)
	logger.Info("test message")

	output := buf.String()
	if !strings.Contains(output, "request_id=req-123") {
		t.Errorf("expected request_id in output, got: %s", output)
	}
}

func TestFromContext_EmptyContext(t *testing.T) {
	var buf bytes.Buffer
	handler := slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	slog.SetDefault(slog.New(handler))

	logger := FromContext(context.Background())
	logger.Info("test message")

	output := buf.String()
	if strings.Contains(output, "request_id") {
		t.Errorf("expected no request_id in output, got: %s", output)
	}
}

func TestFromContext_NilContext(t *testing.T) {
	logger := FromContext(nil)
	if logger == nil {
		t.Error("expected non-nil logger from nil context")
	}
}

func TestConvenienceFunctions_NoPanic(t *testing.T) {
	// Set up a logger to avoid any nil issues.
	var buf bytes.Buffer
	handler := slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	slog.SetDefault(slog.New(handler))

	ctx := context.Background()
	fields := Fields{"key": "value"}

	// These should not panic.
	Info(ctx, "info message", fields)
	Error(ctx, "error message", fields)
	Warn(ctx, "warn message", fields)
	Debug(ctx, "debug message", fields)

	// Also test with nil fields.
	Info(ctx, "info no fields", nil)
}

func TestSetup_JSONFormat(t *testing.T) {
	// Redirect stdout to capture JSON output.
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	cleanup := Setup(Config{
		ServiceName: "json-test",
		Environment: "testing",
		Level:       "info",
		Format:      "json",
	})

	slog.Info("json test message", "foo", "bar")

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	cleanup()

	// Verify it's valid JSON.
	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) == 0 {
		t.Fatal("expected at least one line of JSON output")
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(lines[0]), &parsed); err != nil {
		t.Fatalf("expected valid JSON, got error: %v, output: %s", err, lines[0])
	}

	if parsed["msg"] != "json test message" {
		t.Errorf("expected msg 'json test message', got %v", parsed["msg"])
	}
	if parsed["service"] != "json-test" {
		t.Errorf("expected service 'json-test', got %v", parsed["service"])
	}
}

func TestWithService(t *testing.T) {
	var buf bytes.Buffer
	handler := slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	slog.SetDefault(slog.New(handler))

	logger := WithService("my-service")
	logger.Info("test message")

	output := buf.String()
	if !strings.Contains(output, "service=my-service") {
		t.Errorf("expected service=my-service in output, got: %s", output)
	}
}

func TestFromContext_AllFields(t *testing.T) {
	var buf bytes.Buffer
	handler := slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	slog.SetDefault(slog.New(handler))

	ctx := context.Background()
	ctx = WithRequestID(ctx, "req-abc")
	ctx = WithUserID(ctx, "user-456")
	ctx = WithCorrelationID(ctx, "corr-789")

	logger := FromContext(ctx)
	logger.Info("full context")

	output := buf.String()
	if !strings.Contains(output, "request_id=req-abc") {
		t.Errorf("missing request_id, got: %s", output)
	}
	if !strings.Contains(output, "user_id=user-456") {
		t.Errorf("missing user_id, got: %s", output)
	}
	if !strings.Contains(output, "correlation_id=corr-789") {
		t.Errorf("missing correlation_id, got: %s", output)
	}
}
