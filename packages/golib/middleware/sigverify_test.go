package middleware

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func signRequest(t *testing.T, method, path string, body []byte, secret []byte) string {
	t.Helper()
	timestamp := time.Now().Unix()
	bodyHash := sha256.Sum256(body)
	message := fmt.Sprintf("%s\n%s\n%d\n%s",
		method, path, timestamp,
		hex.EncodeToString(bodyHash[:]),
	)
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(message))
	sig := hex.EncodeToString(mac.Sum(nil))
	return fmt.Sprintf("algorithm=hmac-sha256,timestamp=%d,signature=%s", timestamp, sig)
}

var sigTestSecret = []byte("test-secret-32-bytes-long-enough")

func TestSignVerify_ValidSignature(t *testing.T) {
	handler := SignVerify(SignVerifyConfig{Secret: sigTestSecret})(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}),
	)

	body := []byte(`{"name":"test"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/test", bytes.NewReader(body))
	req.Header.Set("X-Signature", signRequest(t, http.MethodPost, "/api/test", body, sigTestSecret))

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSignVerify_MissingSignature(t *testing.T) {
	handler := SignVerify(SignVerifyConfig{Secret: sigTestSecret})(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}),
	)

	req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestSignVerify_WrongSecret(t *testing.T) {
	handler := SignVerify(SignVerifyConfig{Secret: sigTestSecret})(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}),
	)

	body := []byte(`{}`)
	wrongSecret := []byte("wrong-secret-32-bytes-long-00000")
	req := httptest.NewRequest(http.MethodPost, "/api/test", bytes.NewReader(body))
	req.Header.Set("X-Signature", signRequest(t, http.MethodPost, "/api/test", body, wrongSecret))

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestSignVerify_ExpiredTimestamp(t *testing.T) {
	handler := SignVerify(SignVerifyConfig{
		Secret:    sigTestSecret,
		Tolerance: 1 * time.Second,
	})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Create signature with old timestamp.
	oldTimestamp := time.Now().Add(-10 * time.Minute).Unix()
	body := []byte(`{}`)
	bodyHash := sha256.Sum256(body)
	message := fmt.Sprintf("%s\n%s\n%d\n%s",
		http.MethodPost, "/api/test", oldTimestamp,
		hex.EncodeToString(bodyHash[:]),
	)
	mac := hmac.New(sha256.New, sigTestSecret)
	mac.Write([]byte(message))
	sig := hex.EncodeToString(mac.Sum(nil))
	sigHeader := fmt.Sprintf("algorithm=hmac-sha256,timestamp=%d,signature=%s", oldTimestamp, sig)

	req := httptest.NewRequest(http.MethodPost, "/api/test", bytes.NewReader(body))
	req.Header.Set("X-Signature", sigHeader)

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 for expired signature, got %d", w.Code)
	}
}

func TestSignVerify_SkipPaths(t *testing.T) {
	handler := SignVerify(SignVerifyConfig{
		Secret:    sigTestSecret,
		SkipPaths: []string{"/health", "/ready"},
	})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// No signature needed for skipped paths.
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200 for skipped path, got %d", w.Code)
	}
}

func TestSignVerify_EmptyBody(t *testing.T) {
	handler := SignVerify(SignVerifyConfig{Secret: sigTestSecret})(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}),
	)

	req := httptest.NewRequest(http.MethodGet, "/api/data", nil)
	req.Header.Set("X-Signature", signRequest(t, http.MethodGet, "/api/data", nil, sigTestSecret))

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSignVerify_InvalidAlgorithm(t *testing.T) {
	handler := SignVerify(SignVerifyConfig{Secret: sigTestSecret})(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}),
	)

	req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
	req.Header.Set("X-Signature", "algorithm=sha1,timestamp=1234,signature=abcd")

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestSignVerify_BodyPreserved(t *testing.T) {
	var receivedBody string
	handler := SignVerify(SignVerifyConfig{Secret: sigTestSecret})(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			buf := new(bytes.Buffer)
			buf.ReadFrom(r.Body)
			receivedBody = buf.String()
			w.WriteHeader(http.StatusOK)
		}),
	)

	body := []byte(`{"nik":"3201120509870001"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/verify", bytes.NewReader(body))
	req.Header.Set("X-Signature", signRequest(t, http.MethodPost, "/api/verify", body, sigTestSecret))

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if receivedBody != string(body) {
		t.Errorf("body not preserved: got %q, want %q", receivedBody, string(body))
	}
}

func TestSignVerify_DefaultTolerance(t *testing.T) {
	cfg := SignVerifyConfig{Secret: sigTestSecret}
	// Should use 5 minute default when Tolerance is 0.
	handler := SignVerify(cfg)(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}),
	)

	body := []byte(`{}`)
	req := httptest.NewRequest(http.MethodPost, "/api/test", bytes.NewReader(body))
	req.Header.Set("X-Signature", signRequest(t, http.MethodPost, "/api/test", body, sigTestSecret))

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200 with default tolerance, got %d", w.Code)
	}
}
