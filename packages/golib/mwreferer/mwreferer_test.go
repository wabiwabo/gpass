package mwreferer

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestStrict(t *testing.T) {
	handler := Strict(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Header().Get("Referrer-Policy") != StrictOriginWhenCrossOrigin {
		t.Errorf("got %q", rr.Header().Get("Referrer-Policy"))
	}
}

func TestNone(t *testing.T) {
	handler := None(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Header().Get("Referrer-Policy") != NoReferrer {
		t.Errorf("got %q", rr.Header().Get("Referrer-Policy"))
	}
}

func TestMiddlewareCustom(t *testing.T) {
	policies := []string{
		NoReferrer,
		NoReferrerWhenDowngrade,
		Origin,
		OriginWhenCrossOrigin,
		SameOrigin,
		StrictOrigin,
		StrictOriginWhenCrossOrigin,
		UnsafeURL,
	}
	for _, p := range policies {
		t.Run(p, func(t *testing.T) {
			handler := Middleware(p)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			}))
			req := httptest.NewRequest("GET", "/", nil)
			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)
			if rr.Header().Get("Referrer-Policy") != p {
				t.Errorf("got %q, want %q", rr.Header().Get("Referrer-Policy"), p)
			}
		})
	}
}

func TestMiddlewareEmptyPolicyDefaults(t *testing.T) {
	handler := Middleware("")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Header().Get("Referrer-Policy") != StrictOriginWhenCrossOrigin {
		t.Errorf("default: got %q", rr.Header().Get("Referrer-Policy"))
	}
}

func TestMiddlewarePassthrough(t *testing.T) {
	handler := Strict(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Custom", "yes")
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte("body"))
	}))

	req := httptest.NewRequest("GET", "/", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Errorf("status: %d", rr.Code)
	}
	if rr.Header().Get("X-Custom") != "yes" {
		t.Error("custom header lost")
	}
	if rr.Body.String() != "body" {
		t.Errorf("body: %q", rr.Body.String())
	}
}
