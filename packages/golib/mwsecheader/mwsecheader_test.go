package mwsecheader

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestMiddleware_DefaultHeaders(t *testing.T) {
	mw := Middleware(DefaultConfig())
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	headers := map[string]bool{
		"Content-Security-Policy":   true,
		"Strict-Transport-Security": true,
		"X-Content-Type-Options":    true,
		"X-Frame-Options":           true,
		"Permissions-Policy":        true,
		"Referrer-Policy":           true,
		"Cross-Origin-Opener-Policy":   true,
		"Cross-Origin-Resource-Policy": true,
	}

	for h := range headers {
		if w.Header().Get(h) == "" {
			t.Errorf("missing header: %s", h)
		}
	}
}

func TestMiddleware_Values(t *testing.T) {
	mw := Middleware(DefaultConfig())
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Header().Get("X-Content-Type-Options") != "nosniff" { t.Error("nosniff") }
	if w.Header().Get("X-Frame-Options") != "DENY" { t.Error("DENY") }
}

func TestMiddleware_APIConfig(t *testing.T) {
	mw := Middleware(APIConfig())
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))

	req := httptest.NewRequest("GET", "/api", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	csp := w.Header().Get("Content-Security-Policy")
	if csp != "default-src 'none'; frame-ancestors 'none'" { t.Errorf("CSP = %q", csp) }

	if w.Header().Get("Referrer-Policy") != "no-referrer" { t.Error("referrer") }
}

func TestMiddleware_CustomConfig(t *testing.T) {
	cfg := Config{XContentTypeOptions: "nosniff"}
	mw := Middleware(cfg)
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Header().Get("X-Content-Type-Options") != "nosniff" { t.Error("set") }
	if w.Header().Get("X-Frame-Options") != "" { t.Error("empty should not be set") }
}

func TestSimple(t *testing.T) {
	handler := Simple(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Header().Get("X-Content-Type-Options") != "nosniff" { t.Error("simple") }
}

func TestMiddleware_PassesThrough(t *testing.T) {
	mw := Middleware(DefaultConfig())
	var called bool
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(200)
	}))

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if !called { t.Error("handler not called") }
	if w.Code != 200 { t.Errorf("status = %d", w.Code) }
}

func TestHSTSPreload(t *testing.T) {
	cfg := DefaultConfig()
	mw := Middleware(cfg)
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	hsts := w.Header().Get("Strict-Transport-Security")
	if hsts == "" { t.Fatal("no HSTS") }
	if hsts != "max-age=63072000; includeSubDomains; preload" {
		t.Errorf("HSTS = %q", hsts)
	}
}
