package middleware

import (
	"bytes"
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func captureLog(fn func()) string {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	old := slog.Default()
	slog.SetDefault(logger)
	defer slog.SetDefault(old)

	fn()
	return buf.String()
}

func TestAccessLog_200IsInfo(t *testing.T) {
	handler := AccessLog(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	output := captureLog(func() {
		r := httptest.NewRequest(http.MethodGet, "/test", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
	})

	if !strings.Contains(output, "level=INFO") {
		t.Errorf("expected INFO level, got: %s", output)
	}
	if !strings.Contains(output, "path=/test") {
		t.Errorf("expected path=/test in log, got: %s", output)
	}
	if !strings.Contains(output, "duration=") {
		t.Errorf("expected duration in log, got: %s", output)
	}
}

func TestAccessLog_500IsError(t *testing.T) {
	handler := AccessLog(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))

	output := captureLog(func() {
		r := httptest.NewRequest(http.MethodGet, "/error", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
	})

	if !strings.Contains(output, "level=ERROR") {
		t.Errorf("expected ERROR level, got: %s", output)
	}
}

func TestAccessLog_400IsWarn(t *testing.T) {
	handler := AccessLog(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	}))

	output := captureLog(func() {
		r := httptest.NewRequest(http.MethodGet, "/bad", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
	})

	if !strings.Contains(output, "level=WARN") {
		t.Errorf("expected WARN level, got: %s", output)
	}
}

func TestAccessLog_IncludesRequestID(t *testing.T) {
	handler := AccessLog(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	output := captureLog(func() {
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		ctx := context.WithValue(r.Context(), requestIDKey, "test-req-id")
		r = r.WithContext(ctx)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
	})

	if !strings.Contains(output, "test-req-id") {
		t.Errorf("expected request_id in log, got: %s", output)
	}
}
