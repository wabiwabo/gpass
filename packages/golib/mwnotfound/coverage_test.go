package mwnotfound

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestMiddleware_404WithoutBody pins the branch where the wrapped mux
// returns 404 with no body — middleware must inject the JSON envelope.
func TestMiddleware_404WithoutBody(t *testing.T) {
	mux := http.NewServeMux()
	// Empty mux: any request → default ServeMux 404 ("404 page not found\n").
	// We need a mux that produces 404 WITHOUT writing a body, so use a
	// custom handler that mimics http.NotFound's status without a body.
	mux2 := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})

	h := Middleware(mux2)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/missing", nil))
	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, `"not_found"`) || !strings.Contains(body, `"/missing"`) {
		t.Errorf("body missing JSON envelope: %q", body)
	}
	if rec.Header().Get("Content-Type") != "application/json" {
		t.Errorf("Content-Type = %q", rec.Header().Get("Content-Type"))
	}
	_ = mux
}

// TestMiddleware_PassThroughFor200 pins the branch where the wrapped mux
// returns 200 — middleware must NOT touch the body or headers.
func TestMiddleware_PassThroughFor200(t *testing.T) {
	mux := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(200)
		w.Write([]byte("OK"))
	})
	h := Middleware(mux)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/ok", nil))
	if rec.Code != 200 || rec.Body.String() != "OK" {
		t.Errorf("code=%d body=%q", rec.Code, rec.Body)
	}
}

// TestMiddleware_404WithBody_NotInjected pins the bodyWritten guard:
// when the mux already wrote a body alongside 404, the middleware must
// NOT inject the JSON envelope (would corrupt the existing payload).
func TestMiddleware_404WithBody_NotInjected(t *testing.T) {
	mux := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.NotFound(w, nil) // writes "404 page not found\n" body
	})
	h := Middleware(mux)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/x", nil))
	if rec.Code != 404 {
		t.Errorf("code = %d", rec.Code)
	}
	if strings.Contains(rec.Body.String(), `"not_found"`) {
		t.Errorf("middleware should not inject when body already written: %q", rec.Body)
	}
}

// TestHandler_CustomMessage pins the Handler() factory.
func TestHandler_CustomMessage(t *testing.T) {
	h := Handler("custom not-found message")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/x", nil))
	if !strings.Contains(rec.Body.String(), "custom not-found message") {
		t.Errorf("body = %q", rec.Body)
	}
}

// TestJSON_Default pins the JSON() factory.
func TestJSON_Default(t *testing.T) {
	h := JSON()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/abc/def", nil))
	if rec.Code != 404 {
		t.Errorf("code = %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `"/abc/def"`) {
		t.Errorf("path not in body: %q", rec.Body)
	}
}
