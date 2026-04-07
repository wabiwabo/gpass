package httpclient

import (
	"context"
	"encoding/json"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/garudapass/gpass/packages/golib/circuitbreaker"
)

// TestPost_MarshalError covers the json.Marshal failure branch in Post.
// math.Inf is not encodable as JSON, so json.Marshal returns an error
// and Post must wrap it as "marshal request body".
func TestPost_MarshalError(t *testing.T) {
	c := New("http://example.invalid")
	err := c.Post(context.Background(), "/x", math.Inf(1), nil)
	if err == nil {
		t.Fatal("expected marshal error")
	}
	if !strings.Contains(err.Error(), "marshal request body") {
		t.Errorf("err = %q", err.Error())
	}
}

// TestPost_BadURL covers the http.NewRequestWithContext failure branch
// in Post (invalid control character in URL → request creation fails).
func TestPost_BadURL(t *testing.T) {
	c := New("http://example.com")
	// A NUL byte in the path causes net/http to reject the URL.
	err := c.Post(context.Background(), "/\x00bad", map[string]int{"a": 1}, nil)
	if err == nil {
		t.Fatal("expected create-request error")
	}
	if !strings.Contains(err.Error(), "create request") {
		t.Errorf("err = %q", err.Error())
	}
}

// TestGet_BadURL covers the same branch for Get.
func TestGet_BadURL(t *testing.T) {
	c := New("http://example.com")
	err := c.Get(context.Background(), "/\x00bad", nil)
	if err == nil {
		t.Fatal("expected create-request error")
	}
}

// TestDo_5xxRecordsFailureAndReturnsError covers the server-error branch:
// status >= 500 → circuit breaker records failure → wrapped error
// includes "server error" plus the response body.
func TestDo_5xxRecordsFailureAndReturnsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("oops"))
	}))
	defer srv.Close()

	cb := circuitbreaker.New(100, time.Second)
	c := New(srv.URL, WithCircuitBreaker(cb), WithAPIKey("k"))

	err := c.Get(context.Background(), "/", nil)
	if err == nil || !strings.Contains(err.Error(), "server error: status 500") {
		t.Errorf("err = %v", err)
	}
	if !strings.Contains(err.Error(), "oops") {
		t.Errorf("response body not surfaced in err: %v", err)
	}
}

// TestDo_4xxReturnsClientErrorButNotCBFailure covers the client-error
// branch: status >= 400 must NOT trip the circuit breaker, only return
// the wrapped error. This is a critical contract — a 401 from one caller
// must not break the breaker for everyone.
func TestDo_4xxReturnsClientErrorButNotCBFailure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "no", http.StatusUnauthorized)
	}))
	defer srv.Close()

	cb := circuitbreaker.New(1, time.Second)
	c := New(srv.URL, WithCircuitBreaker(cb))

	err := c.Get(context.Background(), "/", nil)
	if err == nil || !strings.Contains(err.Error(), "client error: status 401") {
		t.Errorf("err = %v", err)
	}
	// CB must still allow because 4xx is success from the caller's POV.
	if !cb.Allow() {
		t.Error("circuit breaker tripped on 4xx — should only trip on 5xx/transport errors")
	}
}

// TestDo_CircuitBreakerOpen short-circuits before any HTTP call. This
// covers the cb.Allow()==false branch.
func TestDo_CircuitBreakerOpen(t *testing.T) {
	cb := circuitbreaker.New(1, time.Second)
	// Trip the breaker by recording failures past the threshold.
	cb.RecordFailure()
	cb.RecordFailure()

	c := New("http://example.invalid", WithCircuitBreaker(cb))
	err := c.Get(context.Background(), "/", nil)
	if err == nil || !strings.Contains(err.Error(), "circuit breaker is open") {
		t.Errorf("err = %v", err)
	}
}

// TestDo_DecodeResponseSuccess covers the json.Decoder happy path with
// a non-nil result pointer and a 2xx status.
func TestDo_DecodeResponseSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"name": "audit"})
	}))
	defer srv.Close()

	c := New(srv.URL, WithAPIKey("xyz"))
	var got map[string]string
	if err := c.Get(context.Background(), "/", &got); err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got["name"] != "audit" {
		t.Errorf("decoded body = %v", got)
	}
}

// TestDo_DecodeResponseFailure covers the json.Decoder error wrap
// (server returns invalid JSON despite 200 status).
func TestDo_DecodeResponseFailure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("not json{"))
	}))
	defer srv.Close()

	c := New(srv.URL)
	var got map[string]string
	err := c.Get(context.Background(), "/", &got)
	if err == nil || !strings.Contains(err.Error(), "decode response") {
		t.Errorf("err = %v", err)
	}
}
