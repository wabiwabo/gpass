package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAuditLog_LogsMethodPathStatus(t *testing.T) {
	handler := AuditLog(1024)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
	}))

	output := captureLog(func() {
		r := httptest.NewRequest(http.MethodPost, "/api/v1/identity/register", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
	})

	if !strings.Contains(output, "method=POST") {
		t.Errorf("expected method=POST in log, got: %s", output)
	}
	if !strings.Contains(output, "path=/api/v1/identity/register") {
		t.Errorf("expected path in log, got: %s", output)
	}
	if !strings.Contains(output, "status=201") {
		t.Errorf("expected status=201 in log, got: %s", output)
	}
}

func TestAuditLog_LogsRequestBodyTruncated(t *testing.T) {
	handler := AuditLog(10)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	output := captureLog(func() {
		body := strings.NewReader("This is a very long request body that exceeds max")
		r := httptest.NewRequest(http.MethodPost, "/test", body)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
	})

	if !strings.Contains(output, "This is a ") {
		t.Errorf("expected truncated request body in log, got: %s", output)
	}
	if strings.Contains(output, "exceeds max") {
		t.Errorf("expected body to be truncated, but found full content: %s", output)
	}
}

func TestAuditLog_RedactsAuthorizationHeader(t *testing.T) {
	handler := AuditLog(1024)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	output := captureLog(func() {
		r := httptest.NewRequest(http.MethodGet, "/test", nil)
		r.Header.Set("Authorization", "Bearer secret-token-123")
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
	})

	if strings.Contains(output, "secret-token-123") {
		t.Errorf("Authorization header value should be redacted, got: %s", output)
	}
	if !strings.Contains(output, "[REDACTED]") {
		t.Errorf("expected [REDACTED] in log, got: %s", output)
	}
}

func TestAuditLog_RedactsCookieHeader(t *testing.T) {
	handler := AuditLog(1024)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	output := captureLog(func() {
		r := httptest.NewRequest(http.MethodGet, "/test", nil)
		r.Header.Set("Cookie", "session=abc123secret")
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
	})

	if strings.Contains(output, "abc123secret") {
		t.Errorf("Cookie header value should be redacted, got: %s", output)
	}
	if !strings.Contains(output, "[REDACTED]") {
		t.Errorf("expected [REDACTED] in log, got: %s", output)
	}
}

func TestAuditLog_RedactsXAPIKeyHeader(t *testing.T) {
	handler := AuditLog(1024)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	output := captureLog(func() {
		r := httptest.NewRequest(http.MethodGet, "/test", nil)
		r.Header.Set("X-API-Key", "gp_live_supersecret")
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
	})

	if strings.Contains(output, "gp_live_supersecret") {
		t.Errorf("X-API-Key header value should be redacted, got: %s", output)
	}
	if !strings.Contains(output, "[REDACTED]") {
		t.Errorf("expected [REDACTED] in log, got: %s", output)
	}
}

func TestAuditLog_NonSensitiveHeadersPreserved(t *testing.T) {
	handler := AuditLog(1024)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	output := captureLog(func() {
		r := httptest.NewRequest(http.MethodGet, "/test", nil)
		r.Header.Set("X-Custom-Header", "visible-value")
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
	})

	if !strings.Contains(output, "visible-value") {
		t.Errorf("expected non-sensitive header value in log, got: %s", output)
	}
}

func TestAuditLog_ResponseBodyLoggedTruncated(t *testing.T) {
	handler := AuditLog(12)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("response body that is very long and should be truncated"))
	}))

	output := captureLog(func() {
		r := httptest.NewRequest(http.MethodGet, "/test", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
	})

	if !strings.Contains(output, "response bod") {
		t.Errorf("expected truncated response body in log, got: %s", output)
	}
	if strings.Contains(output, "should be truncated") {
		t.Errorf("expected response body to be truncated, but found full content: %s", output)
	}
}

func TestAuditLog_UserIDIncluded(t *testing.T) {
	handler := AuditLog(1024)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	output := captureLog(func() {
		r := httptest.NewRequest(http.MethodGet, "/test", nil)
		r.Header.Set("X-User-ID", "user-abc-123")
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
	})

	if !strings.Contains(output, "user-abc-123") {
		t.Errorf("expected user_id in log, got: %s", output)
	}
}

func TestAuditLog_LatencyIncluded(t *testing.T) {
	handler := AuditLog(1024)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	output := captureLog(func() {
		r := httptest.NewRequest(http.MethodGet, "/test", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
	})

	if !strings.Contains(output, "latency=") {
		t.Errorf("expected latency in log, got: %s", output)
	}
}

func TestRedactHeaders_Comprehensive(t *testing.T) {
	h := http.Header{}
	h.Set("Authorization", "Bearer token")
	h.Set("Cookie", "session=abc")
	h.Set("Set-Cookie", "session=xyz")
	h.Set("X-API-Key", "key123")
	h.Set("X-Service-Signature", "sig456")
	h.Set("BFF-Session-Secret", "secret789")
	h.Set("Content-Type", "application/json")

	redacted := RedactHeaders(h)

	for _, name := range SensitiveHeaders {
		if redacted.Get(name) != "[REDACTED]" {
			t.Errorf("expected %s to be [REDACTED], got %s", name, redacted.Get(name))
		}
	}

	if redacted.Get("Content-Type") != "application/json" {
		t.Errorf("expected Content-Type to be preserved, got %s", redacted.Get("Content-Type"))
	}

	// Original should not be modified.
	if h.Get("Authorization") != "Bearer token" {
		t.Error("original header was modified")
	}
}
