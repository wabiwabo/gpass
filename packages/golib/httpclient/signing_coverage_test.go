package httpclient

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// TestSignedClient_HappyPath_RoundTrip pins the full Do path including
// JSON body marshal, header set, signature compute, and server-side
// verification round-trip via VerifySignature.
func TestSignedClient_HappyPath_RoundTrip(t *testing.T) {
	secret := []byte("super-secret-key")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := VerifySignature(r, secret, 30*time.Second); err != nil {
			t.Errorf("server VerifySignature: %v", err)
			http.Error(w, err.Error(), 401)
			return
		}
		if r.Header.Get("X-API-Key") != "key-123" {
			t.Errorf("X-API-Key not propagated")
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("Content-Type not set")
		}
		w.WriteHeader(200)
	}))
	defer srv.Close()

	c := NewSignedClient("key-123", secret, 5*time.Second)
	resp, err := c.Do(context.Background(), "POST", srv.URL+"/api/x", map[string]string{"k": "v"})
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 200 {
		t.Errorf("status = %d", resp.StatusCode)
	}
}

// TestSignedClient_NoBody pins the body==nil branch (no Content-Type).
func TestSignedClient_NoBody(t *testing.T) {
	secret := []byte("k")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Content-Type") != "" {
			t.Errorf("Content-Type should be unset for nil body")
		}
		_ = VerifySignature(r, secret, time.Minute)
	}))
	defer srv.Close()
	c := NewSignedClient("", secret, time.Second)
	if _, err := c.Do(context.Background(), "GET", srv.URL+"/x", nil); err != nil {
		t.Fatal(err)
	}
}

// TestSignedClient_MarshalError pins the json.Marshal failure branch
// (channels can't be marshaled).
func TestSignedClient_MarshalError(t *testing.T) {
	c := NewSignedClient("k", []byte("s"), time.Second)
	_, err := c.Do(context.Background(), "POST", "http://x", make(chan int))
	if err == nil || !strings.Contains(err.Error(), "marshal request body") {
		t.Errorf("err = %v", err)
	}
}

// TestSignedClient_BadURL pins the http.NewRequestWithContext error branch.
func TestSignedClient_BadURL(t *testing.T) {
	c := NewSignedClient("k", []byte("s"), time.Second)
	_, err := c.Do(context.Background(), "BAD METHOD", "http://x", nil)
	if err == nil {
		t.Error("expected error for bad method")
	}
}

// TestVerifySignature_AllRejectionBranches pins each VerifySignature
// rejection: missing header, bad algo, missing/invalid ts, missing sig,
// expired, mismatch, undecodable hex.
func TestVerifySignature_AllRejectionBranches(t *testing.T) {
	secret := []byte("s")
	mk := func(hdr string) *http.Request {
		r := httptest.NewRequest("GET", "/x", nil)
		if hdr != "" {
			r.Header.Set("X-Signature", hdr)
		}
		return r
	}

	cases := []struct {
		name string
		hdr  string
		want string
	}{
		{"missing header", "", "missing X-Signature"},
		{"bad algo", "algorithm=md5,timestamp=1,signature=ab", "unsupported"},
		{"missing ts", "algorithm=hmac-sha256,signature=ab", "missing timestamp"},
		{"bad ts", "algorithm=hmac-sha256,timestamp=abc,signature=ab", "invalid timestamp"},
		{"missing sig", "algorithm=hmac-sha256,timestamp=1", "missing signature"},
		{"expired", fmt.Sprintf("algorithm=hmac-sha256,timestamp=%d,signature=ab", time.Now().Unix()-3600), "expired"},
		{"sig not hex", fmt.Sprintf("algorithm=hmac-sha256,timestamp=%d,signature=zz", time.Now().Unix()), "decode actual"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := VerifySignature(mk(tc.hdr), secret, 30*time.Second)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Errorf("err = %v, want substring %q", err, tc.want)
			}
		})
	}
}

// TestVerifySignature_TamperedBodyRejected pins that mutating the body
// after the signature is computed causes verification to fail.
func TestVerifySignature_TamperedBodyRejected(t *testing.T) {
	secret := []byte("s")
	body := []byte(`{"x":1}`)
	ts := time.Now().Unix()
	sig := computeSignature(secret, "POST", "/api", ts, body)

	r := httptest.NewRequest("POST", "/api", bytes.NewReader([]byte(`{"x":2}`))) // mutated
	r.Header.Set("X-Signature", fmt.Sprintf("algorithm=hmac-sha256,timestamp=%d,signature=%s", ts, sig))
	if err := VerifySignature(r, secret, time.Minute); err == nil || !strings.Contains(err.Error(), "mismatch") {
		t.Errorf("tampered accepted: %v", err)
	}
	// Body should still be readable downstream.
	got, _ := io.ReadAll(r.Body)
	if string(got) != `{"x":2}` {
		t.Errorf("body restoration failed: %q", got)
	}
}
