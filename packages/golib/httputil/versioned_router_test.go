package httputil

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestVersionedRouterRoutesToCorrectVersion(t *testing.T) {
	router := NewVersionedRouter("v1")
	router.Version("v1").HandleFunc("GET /hello", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "hello v1")
	})
	router.Version("v2").HandleFunc("GET /hello", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "hello v2")
	})

	req := httptest.NewRequest(http.MethodGet, "/hello", nil)
	req.Header.Set("Accept", "application/vnd.garudapass.v2+json")
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Body.String() != "hello v2" {
		t.Errorf("expected 'hello v2', got %q", rr.Body.String())
	}
}

func TestVersionedRouterFallsBackToDefault(t *testing.T) {
	router := NewVersionedRouter("v1")
	router.Version("v1").HandleFunc("GET /hello", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "hello v1")
	})

	req := httptest.NewRequest(http.MethodGet, "/hello", nil)
	// No Accept header — should fall back to v1
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Body.String() != "hello v1" {
		t.Errorf("expected 'hello v1', got %q", rr.Body.String())
	}
}

func TestVersionedRouterUnknownVersionReturns406(t *testing.T) {
	router := NewVersionedRouter("v1")
	router.Version("v1").HandleFunc("GET /hello", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "hello v1")
	})

	req := httptest.NewRequest(http.MethodGet, "/hello", nil)
	req.Header.Set("Accept", "application/vnd.garudapass.v99+json")
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotAcceptable {
		t.Errorf("expected 406, got %d", rr.Code)
	}
}

func TestVersionedRouterDifferentHandlersPerVersion(t *testing.T) {
	router := NewVersionedRouter("v1")
	router.Version("v1").HandleFunc("GET /users", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"users":["alice"]}`)
	})
	router.Version("v2").HandleFunc("GET /users", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"data":{"users":[{"name":"alice"}]}}`)
	})

	tests := []struct {
		version  string
		expected string
	}{
		{"v1", `{"users":["alice"]}`},
		{"v2", `{"data":{"users":[{"name":"alice"}]}}`},
	}

	for _, tc := range tests {
		req := httptest.NewRequest(http.MethodGet, "/users", nil)
		req.Header.Set("Accept", "application/vnd.garudapass."+tc.version+"+json")
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		if rr.Body.String() != tc.expected {
			t.Errorf("version %s: expected %q, got %q", tc.version, tc.expected, rr.Body.String())
		}
	}
}

func TestVersionMuxSupportsStandardGoRouting(t *testing.T) {
	router := NewVersionedRouter("v1")
	mux := router.Version("v1")
	mux.HandleFunc("GET /items/{id}", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		fmt.Fprintf(w, "item:%s", id)
	})

	req := httptest.NewRequest(http.MethodGet, "/items/42", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Body.String() != "item:42" {
		t.Errorf("expected 'item:42', got %q", rr.Body.String())
	}
}

func TestVersionReturnsSameMux(t *testing.T) {
	router := NewVersionedRouter("v1")
	mux1 := router.Version("v1")
	mux2 := router.Version("v1")
	if mux1 != mux2 {
		t.Error("expected same mux for same version")
	}
}
