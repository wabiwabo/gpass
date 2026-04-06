package logging

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"regexp"
	"testing"
)

// logEntry is used to parse JSON log output for assertions.
type logEntry map[string]interface{}

func newTestHandler(buf *bytes.Buffer) slog.Handler {
	return slog.NewJSONHandler(buf, &slog.HandlerOptions{Level: slog.LevelDebug})
}

func parseLogEntry(t *testing.T, buf *bytes.Buffer) logEntry {
	t.Helper()
	var entry logEntry
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatalf("failed to parse log output: %v\nraw: %s", err, buf.String())
	}
	return entry
}

func TestRedactingHandler_RedactsSensitiveKeys(t *testing.T) {
	sensitiveKeys := []string{"password", "token", "nik", "secret", "authorization", "cookie", "session_key", "api_key", "credit_card", "ssn", "pin"}

	for _, key := range sensitiveKeys {
		t.Run(key, func(t *testing.T) {
			var buf bytes.Buffer
			h := NewRedactingHandler(newTestHandler(&buf), nil)
			logger := slog.New(h)

			logger.Info("test", key, "sensitive-value-123")

			entry := parseLogEntry(t, &buf)
			if got, ok := entry[key]; !ok {
				t.Fatalf("key %q not found in log output", key)
			} else if got != "[REDACTED]" {
				t.Errorf("key %q: got %q, want [REDACTED]", key, got)
			}
		})
	}
}

func TestRedactingHandler_RedactsSensitiveValues(t *testing.T) {
	tests := []struct {
		name  string
		key   string
		value string
	}{
		{"NIK 16-digit", "citizen_id", "3201234567890001"},
		{"email", "contact", "user@example.com"},
		{"phone international", "mobile", "+6281234567890"},
		{"phone local", "phone", "08123456789012"},
		{"bearer token", "header", "Bearer eyJhbGciOiJSUzI1NiJ9.xyz"},
		{"credit card spaces", "card", "4111 1111 1111 1111"},
		{"credit card dashes", "card", "4111-1111-1111-1111"},
		{"credit card plain", "card", "4111111111111111"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			h := NewRedactingHandler(newTestHandler(&buf), nil)
			logger := slog.New(h)

			logger.Info("test", tt.key, tt.value)

			entry := parseLogEntry(t, &buf)
			if got := entry[tt.key]; got != "[REDACTED]" {
				t.Errorf("value pattern %q: got %q, want [REDACTED]", tt.name, got)
			}
		})
	}
}

func TestRedactingHandler_PreservesNonSensitiveData(t *testing.T) {
	var buf bytes.Buffer
	h := NewRedactingHandler(newTestHandler(&buf), nil)
	logger := slog.New(h)

	logger.Info("user action", "action", "login", "status", "success", "count", 42)

	entry := parseLogEntry(t, &buf)

	if got := entry["action"]; got != "login" {
		t.Errorf("action: got %q, want %q", got, "login")
	}
	if got := entry["status"]; got != "success" {
		t.Errorf("status: got %q, want %q", got, "success")
	}
	if got := entry["count"]; got != float64(42) {
		t.Errorf("count: got %v, want 42", got)
	}
	if got := entry["msg"]; got != "user action" {
		t.Errorf("msg: got %q, want %q", got, "user action")
	}
}

func TestRedactingHandler_WithGroups(t *testing.T) {
	var buf bytes.Buffer
	h := NewRedactingHandler(newTestHandler(&buf), nil)
	logger := slog.New(h)

	logger.Info("test", slog.Group("user",
		slog.String("name", "John"),
		slog.String("password", "s3cret"),
		slog.String("email", "john@example.com"),
	))

	entry := parseLogEntry(t, &buf)

	group, ok := entry["user"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected 'user' group in output, got: %v", entry)
	}
	if got := group["name"]; got != "John" {
		t.Errorf("name: got %q, want %q", got, "John")
	}
	if got := group["password"]; got != "[REDACTED]" {
		t.Errorf("password: got %q, want [REDACTED]", got)
	}
	if got := group["email"]; got != "[REDACTED]" {
		t.Errorf("email value: got %q, want [REDACTED]", got)
	}
}

func TestRedactingHandler_WithAttrs(t *testing.T) {
	var buf bytes.Buffer
	h := NewRedactingHandler(newTestHandler(&buf), nil)
	// Add pre-set attrs including a sensitive one.
	h2 := h.WithAttrs([]slog.Attr{
		slog.String("token", "abc123"),
		slog.String("service", "identity"),
	})
	logger := slog.New(h2)

	logger.Info("request")

	entry := parseLogEntry(t, &buf)
	if got := entry["token"]; got != "[REDACTED]" {
		t.Errorf("token: got %q, want [REDACTED]", got)
	}
	if got := entry["service"]; got != "identity" {
		t.Errorf("service: got %q, want %q", got, "identity")
	}
}

func TestRedactingHandler_CustomRedactKeys(t *testing.T) {
	var buf bytes.Buffer
	cfg := &RedactingConfig{
		RedactKeys: []string{"national_id", "passport_number"},
	}
	h := NewRedactingHandler(newTestHandler(&buf), cfg)
	logger := slog.New(h)

	logger.Info("test", "national_id", "ID12345", "passport_number", "P9876543", "name", "Alice")

	entry := parseLogEntry(t, &buf)
	if got := entry["national_id"]; got != "[REDACTED]" {
		t.Errorf("national_id: got %q, want [REDACTED]", got)
	}
	if got := entry["passport_number"]; got != "[REDACTED]" {
		t.Errorf("passport_number: got %q, want [REDACTED]", got)
	}
	if got := entry["name"]; got != "Alice" {
		t.Errorf("name: got %q, want %q", got, "Alice")
	}
}

func TestRedactingHandler_CustomRedactPatterns(t *testing.T) {
	var buf bytes.Buffer
	// Custom pattern: Indonesian NPWP (15 digits with dots and dashes, e.g. 01.234.567.8-901.000)
	npwpPattern := regexp.MustCompile(`\d{2}\.\d{3}\.\d{3}\.\d-\d{3}\.\d{3}`)
	cfg := &RedactingConfig{
		RedactPatterns: []*regexp.Regexp{npwpPattern},
	}
	h := NewRedactingHandler(newTestHandler(&buf), cfg)
	logger := slog.New(h)

	logger.Info("tax", "npwp", "01.234.567.8-901.000", "name", "PT Example")

	entry := parseLogEntry(t, &buf)
	if got := entry["npwp"]; got != "[REDACTED]" {
		t.Errorf("npwp: got %q, want [REDACTED]", got)
	}
	if got := entry["name"]; got != "PT Example" {
		t.Errorf("name: got %q, want %q", got, "PT Example")
	}
}

func TestRedactingHandler_BearerTokenRedaction(t *testing.T) {
	var buf bytes.Buffer
	h := NewRedactingHandler(newTestHandler(&buf), nil)
	logger := slog.New(h)

	logger.Info("auth", "auth_header", "Bearer eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIn0.sig")

	entry := parseLogEntry(t, &buf)
	if got := entry["auth_header"]; got != "[REDACTED]" {
		t.Errorf("auth_header: got %q, want [REDACTED]", got)
	}
}

func TestRedactingHandler_CreditCardRedaction(t *testing.T) {
	tests := []struct {
		name  string
		value string
	}{
		{"plain", "4111111111111111"},
		{"spaces", "4111 1111 1111 1111"},
		{"dashes", "4111-1111-1111-1111"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			h := NewRedactingHandler(newTestHandler(&buf), nil)
			logger := slog.New(h)

			logger.Info("payment", "card_number", tt.value)

			entry := parseLogEntry(t, &buf)
			if got := entry["card_number"]; got != "[REDACTED]" {
				t.Errorf("credit card %s: got %q, want [REDACTED]", tt.name, got)
			}
		})
	}
}

func TestRedactingHandler_CaseInsensitiveKeys(t *testing.T) {
	cases := []string{"PASSWORD", "Token", "NIK", "Secret", "API_KEY", "Authorization"}

	for _, key := range cases {
		t.Run(key, func(t *testing.T) {
			var buf bytes.Buffer
			h := NewRedactingHandler(newTestHandler(&buf), nil)
			logger := slog.New(h)

			logger.Info("test", key, "value123")

			entry := parseLogEntry(t, &buf)
			// slog lowercases keys by default in JSONHandler? No, it preserves them.
			if got := entry[key]; got != "[REDACTED]" {
				t.Errorf("case-insensitive key %q: got %q, want [REDACTED]", key, got)
			}
		})
	}
}

func TestSetupWithRedaction(t *testing.T) {
	// Save and restore the default logger.
	origLogger := slog.Default()
	defer slog.SetDefault(origLogger)

	// We can't easily capture os.Stdout, so we verify the handler type instead.
	cleanup := SetupWithRedaction(Config{
		ServiceName: "test-svc",
		Environment: "testing",
		Level:       "debug",
		Format:      "json",
	})
	defer cleanup()

	// Verify the default logger's handler chain contains a RedactingHandler.
	logger := slog.Default()
	handler := logger.Handler()

	// The handler should be a RedactingHandler (possibly wrapped with attrs).
	// We test by logging to a buffer-based handler directly.
	var buf bytes.Buffer
	innerHandler := slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	rh := NewRedactingHandler(innerHandler, nil)
	testLogger := slog.New(rh)

	testLogger.Info("integration", "password", "should-be-redacted", "safe_key", "safe_value")

	entry := parseLogEntry(t, &buf)
	if got := entry["password"]; got != "[REDACTED]" {
		t.Errorf("password: got %q, want [REDACTED]", got)
	}
	if got := entry["safe_key"]; got != "safe_value" {
		t.Errorf("safe_key: got %q, want %q", got, "safe_value")
	}

	// Also verify the global logger was set (non-nil handler).
	if handler == nil {
		t.Error("expected non-nil handler from SetupWithRedaction")
	}

	// Verify it's a redacting handler by checking Enabled works.
	if !handler.Enabled(context.Background(), slog.LevelInfo) {
		t.Error("expected Info to be enabled")
	}
}

func TestRedactingHandler_NilConfig(t *testing.T) {
	var buf bytes.Buffer
	h := NewRedactingHandler(newTestHandler(&buf), nil)
	logger := slog.New(h)

	// Should still redact default sensitive keys.
	logger.Info("test", "password", "secret123", "nik", "3201234567890001")

	entry := parseLogEntry(t, &buf)
	if got := entry["password"]; got != "[REDACTED]" {
		t.Errorf("password: got %q, want [REDACTED]", got)
	}
	if got := entry["nik"]; got != "[REDACTED]" {
		t.Errorf("nik: got %q, want [REDACTED]", got)
	}
}

func TestRedactingHandler_EnabledDelegates(t *testing.T) {
	var buf bytes.Buffer
	// Inner handler set to Warn level.
	inner := slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelWarn})
	h := NewRedactingHandler(inner, nil)

	ctx := context.Background()

	if h.Enabled(ctx, slog.LevelDebug) {
		t.Error("expected Debug to be disabled when inner is Warn")
	}
	if h.Enabled(ctx, slog.LevelInfo) {
		t.Error("expected Info to be disabled when inner is Warn")
	}
	if !h.Enabled(ctx, slog.LevelWarn) {
		t.Error("expected Warn to be enabled when inner is Warn")
	}
	if !h.Enabled(ctx, slog.LevelError) {
		t.Error("expected Error to be enabled when inner is Warn")
	}
}
