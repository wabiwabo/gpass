package timeout

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// TestMiddleware_DoubleWriteHeaderIgnored pins the wroteHeader guard in
// timeoutWriter.WriteHeader — second call must be a no-op.
func TestMiddleware_DoubleWriteHeaderIgnored(t *testing.T) {
	mw := Middleware(time.Second)
	h := mw(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(201)
		w.WriteHeader(500) // must be ignored
	}))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/", nil))
	if rec.Code != 201 {
		t.Errorf("status = %d, want 201 (second WriteHeader should be ignored)", rec.Code)
	}
}

// TestMiddleware_ImplicitWriteHeaderViaWrite pins the Write branch where
// the handler calls Write directly without WriteHeader (the wroteHeader=
// false → true transition inside Write).
func TestMiddleware_ImplicitWriteHeaderViaWrite(t *testing.T) {
	mw := Middleware(time.Second)
	h := mw(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte("hello"))
	}))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/", nil))
	if rec.Body.String() != "hello" {
		t.Errorf("body = %q", rec.Body.String())
	}
}

// TestMiddleware_TimesOut pins the ctx.Done branch: a slow handler must
// be interrupted with a 504 problem+json response.
func TestMiddleware_TimesOut_Cov(t *testing.T) {
	mw := Middleware(20 * time.Millisecond)
	h := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
		case <-time.After(time.Second):
		}
	}))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/", nil))
	if rec.Code != http.StatusGatewayTimeout {
		t.Errorf("status = %d, want 504", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/problem+json" {
		t.Errorf("Content-Type = %q", ct)
	}
	if !strings.Contains(rec.Body.String(), "Request timed out") {
		t.Errorf("body missing detail: %q", rec.Body.String())
	}
}

// TestMiddleware_DoubleWriteAfterHeaderSet pins the wroteHeader=true
// branch in Write — handler sets header explicitly then writes body.
func TestMiddleware_DoubleWriteAfterHeaderSet(t *testing.T) {
	mw := Middleware(time.Second)
	h := mw(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(202)
		w.Write([]byte("a"))
		w.Write([]byte("b"))
	}))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/", nil))
	if rec.Code != 202 || rec.Body.String() != "ab" {
		t.Errorf("code=%d body=%q", rec.Code, rec.Body)
	}
}
