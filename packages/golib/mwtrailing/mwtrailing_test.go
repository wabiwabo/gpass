package mwtrailing

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestStripRedirects(t *testing.T) {
	handler := Strip(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/api/v1/users/", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusMovedPermanently {
		t.Errorf("status: got %d, want 301", rr.Code)
	}
	loc := rr.Header().Get("Location")
	if loc != "/api/v1/users" {
		t.Errorf("Location: got %q, want %q", loc, "/api/v1/users")
	}
}

func TestStripPreservesQuery(t *testing.T) {
	handler := Strip(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/api/?page=2", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	loc := rr.Header().Get("Location")
	if loc != "/api?page=2" {
		t.Errorf("Location: got %q", loc)
	}
}

func TestStripSkipsRoot(t *testing.T) {
	var called bool
	handler := Strip(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("root path should pass through, got %d", rr.Code)
	}
	if !called {
		t.Error("handler should have been called")
	}
}

func TestStripNoTrailing(t *testing.T) {
	var called bool
	handler := Strip(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/api/v1/users", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("no-trailing should pass through, got %d", rr.Code)
	}
	if !called {
		t.Error("handler should have been called")
	}
}

func TestAddRedirects(t *testing.T) {
	handler := Add(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/api/v1/users", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusMovedPermanently {
		t.Errorf("status: got %d, want 301", rr.Code)
	}
	loc := rr.Header().Get("Location")
	if loc != "/api/v1/users/" {
		t.Errorf("Location: got %q", loc)
	}
}

func TestAddSkipsFileExtension(t *testing.T) {
	var called bool
	handler := Add(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/api/docs/spec.json", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("file ext should pass through, got %d", rr.Code)
	}
	if !called {
		t.Error("handler should have been called")
	}
}

func TestAddSkipsRoot(t *testing.T) {
	handler := Add(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("root should pass through, got %d", rr.Code)
	}
}

func TestStripInPlace(t *testing.T) {
	var gotPath string
	handler := StripInPlace(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/api/v1/", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("status: got %d, want 200", rr.Code)
	}
	if gotPath != "/api/v1" {
		t.Errorf("path: got %q, want %q", gotPath, "/api/v1")
	}
}

func TestStripInPlaceRoot(t *testing.T) {
	var gotPath string
	handler := StripInPlace(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if gotPath != "/" {
		t.Errorf("root should be preserved: got %q", gotPath)
	}
}
