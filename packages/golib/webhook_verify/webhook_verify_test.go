package webhook_verify

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"
)

var testSecret = []byte("webhook-secret-for-testing")

func TestSign_Deterministic(t *testing.T) {
	s1 := Sign(testSecret, "1700000000", []byte(`{"event":"test"}`))
	s2 := Sign(testSecret, "1700000000", []byte(`{"event":"test"}`))
	if s1 != s2 {
		t.Error("Sign should be deterministic")
	}
}

func TestSign_DifferentInputs(t *testing.T) {
	body := []byte(`{"event":"test"}`)
	base := Sign(testSecret, "1700000000", body)

	// Different timestamp
	s1 := Sign(testSecret, "1700000001", body)
	if s1 == base {
		t.Error("different timestamp should produce different signature")
	}

	// Different body
	s2 := Sign(testSecret, "1700000000", []byte(`{"event":"other"}`))
	if s2 == base {
		t.Error("different body should produce different signature")
	}

	// Different secret
	s3 := Sign([]byte("other-secret"), "1700000000", body)
	if s3 == base {
		t.Error("different secret should produce different signature")
	}
}

func TestSign_HexOutput(t *testing.T) {
	sig := Sign(testSecret, "1700000000", []byte("test"))
	if len(sig) != 64 {
		t.Errorf("sig len = %d, want 64 (SHA-256 hex)", len(sig))
	}
	for _, c := range sig {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			t.Errorf("non-hex char: %c", c)
		}
	}
}

func TestVerify_ValidTimestamp(t *testing.T) {
	ts := strconv.FormatInt(time.Now().Unix(), 10)
	result := Verify(DefaultConfig(testSecret), ts, []byte("body"))
	if !result.Valid {
		t.Errorf("valid timestamp rejected: %s", result.Error)
	}
}

func TestVerify_ExpiredTimestamp(t *testing.T) {
	ts := strconv.FormatInt(time.Now().Add(-10*time.Minute).Unix(), 10)
	result := Verify(DefaultConfig(testSecret), ts, []byte("body"))
	if result.Valid {
		t.Error("expired timestamp should be rejected")
	}
	if !strings.Contains(result.Error, "too old") {
		t.Errorf("Error = %q", result.Error)
	}
}

func TestVerify_FutureTimestamp(t *testing.T) {
	ts := strconv.FormatInt(time.Now().Add(10*time.Minute).Unix(), 10)
	result := Verify(DefaultConfig(testSecret), ts, []byte("body"))
	if result.Valid {
		t.Error("future timestamp should be rejected")
	}
}

func TestVerify_InvalidTimestampFormat(t *testing.T) {
	result := Verify(DefaultConfig(testSecret), "not-a-number", []byte("body"))
	if result.Valid {
		t.Error("invalid format should be rejected")
	}
	if !strings.Contains(result.Error, "invalid timestamp") {
		t.Errorf("Error = %q", result.Error)
	}
}

func TestVerifySignature_Valid(t *testing.T) {
	cfg := DefaultConfig(testSecret)
	ts := strconv.FormatInt(time.Now().Unix(), 10)
	body := []byte(`{"event":"user.created","user_id":"123"}`)
	sig := Sign(testSecret, ts, body)

	result := VerifySignature(cfg, ts, sig, body)
	if !result.Valid {
		t.Errorf("valid signature rejected: %s", result.Error)
	}
	if result.Timestamp.IsZero() {
		t.Error("Timestamp should be set")
	}
}

func TestVerifySignature_InvalidSignature(t *testing.T) {
	cfg := DefaultConfig(testSecret)
	ts := strconv.FormatInt(time.Now().Unix(), 10)
	body := []byte(`{"event":"test"}`)

	result := VerifySignature(cfg, ts, "invalid-signature", body)
	if result.Valid {
		t.Error("invalid signature should be rejected")
	}
	if result.Error != "invalid signature" {
		t.Errorf("Error = %q", result.Error)
	}
}

func TestVerifySignature_TamperedBody(t *testing.T) {
	cfg := DefaultConfig(testSecret)
	ts := strconv.FormatInt(time.Now().Unix(), 10)
	body := []byte(`{"event":"test"}`)
	sig := Sign(testSecret, ts, body)

	// Tamper with body
	tampered := []byte(`{"event":"tampered"}`)
	result := VerifySignature(cfg, ts, sig, tampered)
	if result.Valid {
		t.Error("tampered body should be rejected")
	}
}

func TestVerifySignature_WrongSecret(t *testing.T) {
	cfg := DefaultConfig([]byte("correct-secret"))
	ts := strconv.FormatInt(time.Now().Unix(), 10)
	body := []byte("data")
	sig := Sign([]byte("wrong-secret"), ts, body)

	result := VerifySignature(cfg, ts, sig, body)
	if result.Valid {
		t.Error("wrong secret should fail")
	}
}

func TestVerifySignature_ExpiredTimestamp(t *testing.T) {
	cfg := DefaultConfig(testSecret)
	ts := strconv.FormatInt(time.Now().Add(-10*time.Minute).Unix(), 10)
	body := []byte("data")
	sig := Sign(testSecret, ts, body)

	result := VerifySignature(cfg, ts, sig, body)
	if result.Valid {
		t.Error("expired timestamp should fail before signature check")
	}
}

func TestDefaultConfig_Values(t *testing.T) {
	cfg := DefaultConfig(testSecret)
	if cfg.SignatureHeader != "X-Webhook-Signature" {
		t.Errorf("SignatureHeader = %q", cfg.SignatureHeader)
	}
	if cfg.TimestampHeader != "X-Webhook-Timestamp" {
		t.Errorf("TimestampHeader = %q", cfg.TimestampHeader)
	}
	if cfg.MaxAge != 5*time.Minute {
		t.Errorf("MaxAge = %v", cfg.MaxAge)
	}
}

func TestMiddleware_MissingSignature(t *testing.T) {
	mw := Middleware(DefaultConfig(testSecret))
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))

	req := httptest.NewRequest("POST", "/webhook", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != 401 {
		t.Errorf("status = %d, want 401", w.Code)
	}
}

func TestMiddleware_MissingTimestamp(t *testing.T) {
	mw := Middleware(DefaultConfig(testSecret))
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))

	req := httptest.NewRequest("POST", "/webhook", nil)
	req.Header.Set("X-Webhook-Signature", "some-sig")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != 401 {
		t.Errorf("status = %d, want 401", w.Code)
	}
}

func TestVerify_ZeroMaxAge(t *testing.T) {
	cfg := Config{MaxAge: 0}
	ts := strconv.FormatInt(time.Now().Add(-1*time.Hour).Unix(), 10)
	result := Verify(cfg, ts, []byte("body"))
	if !result.Valid {
		t.Error("zero MaxAge should skip age check")
	}
}
