package reqsign

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"
)

func TestSigner_Sign(t *testing.T) {
	signer := NewSigner([]byte("secret-key"))

	req := httptest.NewRequest(http.MethodPost, "/api/users", nil)
	signer.Sign(req)

	if req.Header.Get(HeaderSignature) == "" {
		t.Error("should set signature header")
	}
	if req.Header.Get(HeaderTimestamp) == "" {
		t.Error("should set timestamp header")
	}
}

func TestVerifier_ValidSignature(t *testing.T) {
	secret := []byte("shared-secret")
	signer := NewSigner(secret)
	verifier := NewVerifier(secret, 5*time.Minute)

	req := httptest.NewRequest(http.MethodGet, "/api/data", nil)
	signer.Sign(req)

	if err := verifier.Verify(req); err != nil {
		t.Errorf("valid signature should verify: %v", err)
	}
}

func TestVerifier_WrongSecret(t *testing.T) {
	signer := NewSigner([]byte("secret-a"))
	verifier := NewVerifier([]byte("secret-b"), 5*time.Minute)

	req := httptest.NewRequest(http.MethodGet, "/api/data", nil)
	signer.Sign(req)

	if err := verifier.Verify(req); err == nil {
		t.Error("wrong secret should fail")
	}
}

func TestVerifier_MissingSignature(t *testing.T) {
	verifier := NewVerifier([]byte("secret"), 5*time.Minute)
	req := httptest.NewRequest(http.MethodGet, "/", nil)

	if err := verifier.Verify(req); err == nil {
		t.Error("missing signature should fail")
	}
}

func TestVerifier_MissingTimestamp(t *testing.T) {
	verifier := NewVerifier([]byte("secret"), 5*time.Minute)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set(HeaderSignature, "some-sig")

	if err := verifier.Verify(req); err == nil {
		t.Error("missing timestamp should fail")
	}
}

func TestVerifier_ExpiredTimestamp(t *testing.T) {
	secret := []byte("secret")
	signer := NewSigner(secret)
	verifier := NewVerifier(secret, 1*time.Second)

	req := httptest.NewRequest(http.MethodGet, "/api/data", nil)
	signer.Sign(req)

	// Override timestamp to 10 minutes ago.
	oldTime := strconv.FormatInt(time.Now().Add(-10*time.Minute).Unix(), 10)
	req.Header.Set(HeaderTimestamp, oldTime)

	if err := verifier.Verify(req); err == nil {
		t.Error("expired timestamp should fail")
	}
}

func TestVerifier_TamperedPath(t *testing.T) {
	secret := []byte("secret")
	signer := NewSigner(secret)
	verifier := NewVerifier(secret, 5*time.Minute)

	req := httptest.NewRequest(http.MethodGet, "/api/data", nil)
	signer.Sign(req)

	// Tamper with path.
	req.URL.Path = "/api/admin"

	if err := verifier.Verify(req); err == nil {
		t.Error("tampered path should fail")
	}
}

func TestMiddleware_ValidRequest(t *testing.T) {
	secret := []byte("middleware-secret")
	signer := NewSigner(secret)
	verifier := NewVerifier(secret, 5*time.Minute)

	var called bool
	handler := verifier.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
	signer.Sign(req)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("valid: got %d", w.Code)
	}
	if !called {
		t.Error("handler should be called")
	}
}

func TestMiddleware_InvalidRequest(t *testing.T) {
	verifier := NewVerifier([]byte("secret"), 5*time.Minute)

	handler := verifier.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("should not be called")
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("invalid: got %d", w.Code)
	}
}

func TestRoundTripper(t *testing.T) {
	secret := []byte("rt-secret")
	signer := NewSigner(secret)
	verifier := NewVerifier(secret, 5*time.Minute)

	server := httptest.NewServer(verifier.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})))
	defer server.Close()

	client := &http.Client{
		Transport: &RoundTripper{Signer: signer},
	}

	resp, err := client.Get(server.URL + "/api/data")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("roundtripper: got %d", resp.StatusCode)
	}
}

func TestVerifier_DefaultMaxAge(t *testing.T) {
	v := NewVerifier([]byte("secret"), 0) // Should default.
	if v.maxAge != DefaultMaxAge {
		t.Errorf("default max age: got %v", v.maxAge)
	}
}
