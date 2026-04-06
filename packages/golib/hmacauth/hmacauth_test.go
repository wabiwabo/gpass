package hmacauth

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

var testSecret = []byte("super-secret-key-for-testing-only")

func TestSign_Deterministic(t *testing.T) {
	cfg := DefaultConfig(testSecret)
	ts := "2024-06-15T10:00:00Z"

	sig1 := Sign(cfg, "GET", "/api/v1/users", ts, http.Header{})
	sig2 := Sign(cfg, "GET", "/api/v1/users", ts, http.Header{})

	if sig1 != sig2 {
		t.Errorf("Sign not deterministic: %q != %q", sig1, sig2)
	}
}

func TestSign_DifferentInputsDifferentSignatures(t *testing.T) {
	cfg := DefaultConfig(testSecret)
	ts := "2024-06-15T10:00:00Z"

	tests := []struct {
		name    string
		method  string
		path    string
		ts      string
	}{
		{"diff method", "POST", "/api/v1/users", ts},
		{"diff path", "GET", "/api/v1/items", ts},
		{"diff timestamp", "GET", "/api/v1/users", "2024-06-15T10:01:00Z"},
	}

	base := Sign(cfg, "GET", "/api/v1/users", ts, http.Header{})
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sig := Sign(cfg, tt.method, tt.path, tt.ts, http.Header{})
			if sig == base {
				t.Error("different inputs should produce different signatures")
			}
		})
	}
}

func TestSign_DifferentSecretsDifferentSignatures(t *testing.T) {
	cfg1 := DefaultConfig([]byte("secret-1"))
	cfg2 := DefaultConfig([]byte("secret-2"))
	ts := "2024-06-15T10:00:00Z"

	sig1 := Sign(cfg1, "GET", "/test", ts, http.Header{})
	sig2 := Sign(cfg2, "GET", "/test", ts, http.Header{})

	if sig1 == sig2 {
		t.Error("different secrets should produce different signatures")
	}
}

func TestSign_WithSignedHeaders(t *testing.T) {
	cfg := Config{
		Secret:        testSecret,
		SignedHeaders: []string{"X-Tenant-ID", "Content-Type"},
	}
	ts := "2024-06-15T10:00:00Z"

	h1 := http.Header{}
	h1.Set("X-Tenant-ID", "tenant-1")
	h1.Set("Content-Type", "application/json")

	h2 := http.Header{}
	h2.Set("X-Tenant-ID", "tenant-2")
	h2.Set("Content-Type", "application/json")

	sig1 := Sign(cfg, "POST", "/api", ts, h1)
	sig2 := Sign(cfg, "POST", "/api", ts, h2)

	if sig1 == sig2 {
		t.Error("different header values should produce different signatures")
	}
}

func TestVerify_Valid(t *testing.T) {
	cfg := DefaultConfig(testSecret)
	ts := "2024-06-15T10:00:00Z"
	sig := Sign(cfg, "POST", "/api/verify", ts, http.Header{})

	if !Verify(cfg, "POST", "/api/verify", ts, sig, http.Header{}) {
		t.Error("valid signature should verify")
	}
}

func TestVerify_Invalid(t *testing.T) {
	cfg := DefaultConfig(testSecret)
	ts := "2024-06-15T10:00:00Z"

	if Verify(cfg, "POST", "/api/verify", ts, "invalid-signature", http.Header{}) {
		t.Error("invalid signature should not verify")
	}
}

func TestVerify_Tampered(t *testing.T) {
	cfg := DefaultConfig(testSecret)
	ts := "2024-06-15T10:00:00Z"
	sig := Sign(cfg, "POST", "/api/verify", ts, http.Header{})

	// Tamper with one character
	tampered := sig[:len(sig)-1] + "0"
	if tampered == sig {
		tampered = sig[:len(sig)-1] + "1"
	}

	if Verify(cfg, "POST", "/api/verify", ts, tampered, http.Header{}) {
		t.Error("tampered signature should not verify")
	}
}

func TestCheckTimestamp_Valid(t *testing.T) {
	ts := time.Now().UTC().Format(time.RFC3339)
	if err := CheckTimestamp(ts, 5*time.Minute); err != nil {
		t.Errorf("valid timestamp rejected: %v", err)
	}
}

func TestCheckTimestamp_TooOld(t *testing.T) {
	ts := time.Now().Add(-10 * time.Minute).UTC().Format(time.RFC3339)
	if err := CheckTimestamp(ts, 5*time.Minute); err == nil {
		t.Error("old timestamp should be rejected")
	}
}

func TestCheckTimestamp_Future(t *testing.T) {
	ts := time.Now().Add(10 * time.Minute).UTC().Format(time.RFC3339)
	if err := CheckTimestamp(ts, 5*time.Minute); err == nil {
		t.Error("future timestamp should be rejected")
	}
}

func TestCheckTimestamp_InvalidFormat(t *testing.T) {
	if err := CheckTimestamp("not-a-timestamp", 5*time.Minute); err == nil {
		t.Error("invalid format should return error")
	}
}

func TestMiddleware_ValidRequest(t *testing.T) {
	cfg := DefaultConfig(testSecret)
	mw := Middleware(cfg)

	var called bool
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(200)
	}))

	req := httptest.NewRequest("GET", "/api/test", nil)
	SignRequest(cfg, req)

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if !called {
		t.Error("handler should be called for valid request")
	}
	if w.Code != 200 {
		t.Errorf("status = %d, want 200", w.Code)
	}
}

func TestMiddleware_MissingSignature(t *testing.T) {
	cfg := DefaultConfig(testSecret)
	mw := Middleware(cfg)

	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called")
	}))

	req := httptest.NewRequest("GET", "/api/test", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != 401 {
		t.Errorf("status = %d, want 401", w.Code)
	}
	if !strings.Contains(w.Body.String(), "missing signature") {
		t.Errorf("body = %q", w.Body.String())
	}
}

func TestMiddleware_MissingTimestamp(t *testing.T) {
	cfg := DefaultConfig(testSecret)
	mw := Middleware(cfg)

	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called")
	}))

	req := httptest.NewRequest("GET", "/api/test", nil)
	req.Header.Set("X-Signature", "some-sig")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != 401 {
		t.Errorf("status = %d, want 401", w.Code)
	}
}

func TestMiddleware_InvalidTimestamp(t *testing.T) {
	cfg := DefaultConfig(testSecret)
	mw := Middleware(cfg)

	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called")
	}))

	req := httptest.NewRequest("GET", "/api/test", nil)
	req.Header.Set("X-Signature", "some-sig")
	req.Header.Set("X-Timestamp", "invalid")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != 401 {
		t.Errorf("status = %d, want 401", w.Code)
	}
}

func TestMiddleware_ExpiredTimestamp(t *testing.T) {
	cfg := DefaultConfig(testSecret)
	mw := Middleware(cfg)

	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called")
	}))

	req := httptest.NewRequest("GET", "/api/test", nil)
	req.Header.Set("X-Signature", "some-sig")
	req.Header.Set("X-Timestamp", time.Now().Add(-1*time.Hour).UTC().Format(time.RFC3339))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != 401 {
		t.Errorf("status = %d, want 401", w.Code)
	}
}

func TestMiddleware_InvalidSignature(t *testing.T) {
	cfg := DefaultConfig(testSecret)
	mw := Middleware(cfg)

	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called")
	}))

	req := httptest.NewRequest("GET", "/api/test", nil)
	req.Header.Set("X-Signature", "deadbeef")
	req.Header.Set("X-Timestamp", time.Now().UTC().Format(time.RFC3339))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != 401 {
		t.Errorf("status = %d, want 401", w.Code)
	}
}

func TestMiddleware_WrongSecret(t *testing.T) {
	serverCfg := DefaultConfig([]byte("server-secret"))
	clientCfg := DefaultConfig([]byte("wrong-secret"))

	mw := Middleware(serverCfg)

	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called")
	}))

	req := httptest.NewRequest("POST", "/api/data", nil)
	SignRequest(clientCfg, req)

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != 401 {
		t.Errorf("status = %d, want 401", w.Code)
	}
}

func TestSignRequest(t *testing.T) {
	cfg := DefaultConfig(testSecret)
	req := httptest.NewRequest("POST", "/api/verify", nil)
	SignRequest(cfg, req)

	if req.Header.Get("X-Signature") == "" {
		t.Error("X-Signature header not set")
	}
	if req.Header.Get("X-Timestamp") == "" {
		t.Error("X-Timestamp header not set")
	}

	// Timestamp should be valid RFC3339
	ts := req.Header.Get("X-Timestamp")
	if _, err := time.Parse(time.RFC3339, ts); err != nil {
		t.Errorf("invalid timestamp format: %q", ts)
	}
}

func TestSignRequest_CustomHeaders(t *testing.T) {
	cfg := Config{
		Secret:          testSecret,
		HeaderName:      "X-Auth-Sig",
		TimestampHeader: "X-Auth-Time",
	}

	req := httptest.NewRequest("GET", "/test", nil)
	SignRequest(cfg, req)

	if req.Header.Get("X-Auth-Sig") == "" {
		t.Error("custom signature header not set")
	}
	if req.Header.Get("X-Auth-Time") == "" {
		t.Error("custom timestamp header not set")
	}
}

func TestDefaultConfig_Values(t *testing.T) {
	cfg := DefaultConfig(testSecret)
	if cfg.HeaderName != "X-Signature" {
		t.Errorf("HeaderName = %q", cfg.HeaderName)
	}
	if cfg.TimestampHeader != "X-Timestamp" {
		t.Errorf("TimestampHeader = %q", cfg.TimestampHeader)
	}
	if cfg.MaxSkew != 5*time.Minute {
		t.Errorf("MaxSkew = %v", cfg.MaxSkew)
	}
}

func TestMiddleware_DefaultHeaders(t *testing.T) {
	// Config with empty header names should use defaults
	cfg := Config{Secret: testSecret}
	mw := Middleware(cfg)

	var called bool
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	SignRequest(DefaultConfig(testSecret), req)

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if !called {
		t.Error("handler should be called with default headers")
	}
}

func TestSign_HexOutput(t *testing.T) {
	cfg := DefaultConfig(testSecret)
	sig := Sign(cfg, "GET", "/test", "2024-01-01T00:00:00Z", http.Header{})

	// SHA-256 HMAC hex is 64 chars
	if len(sig) != 64 {
		t.Errorf("sig len = %d, want 64", len(sig))
	}

	// All hex characters
	for _, c := range sig {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			t.Errorf("non-hex character: %c", c)
		}
	}
}
