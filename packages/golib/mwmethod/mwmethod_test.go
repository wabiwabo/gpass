package mwmethod

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAllowPermitted(t *testing.T) {
	handler := Allow("GET", "POST")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	for _, method := range []string{"GET", "POST"} {
		req := httptest.NewRequest(method, "/", nil)
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Errorf("%s: got %d, want 200", method, rr.Code)
		}
	}
}

func TestAllowRejected(t *testing.T) {
	handler := Allow("GET")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	for _, method := range []string{"POST", "PUT", "DELETE", "PATCH"} {
		req := httptest.NewRequest(method, "/", nil)
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		if rr.Code != http.StatusMethodNotAllowed {
			t.Errorf("%s: got %d, want 405", method, rr.Code)
		}
		allow := rr.Header().Get("Allow")
		if allow == "" {
			t.Errorf("%s: Allow header should be set", method)
		}
	}
}

func TestGET(t *testing.T) {
	handler := GET(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	tests := []struct {
		method string
		want   int
	}{
		{"GET", 200},
		{"HEAD", 200},
		{"POST", 405},
		{"PUT", 405},
	}
	for _, tt := range tests {
		t.Run(tt.method, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, "/", nil)
			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)
			if rr.Code != tt.want {
				t.Errorf("got %d, want %d", rr.Code, tt.want)
			}
		})
	}
}

func TestPOST(t *testing.T) {
	handler := POST(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("POST", "/", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != 200 {
		t.Errorf("POST: got %d, want 200", rr.Code)
	}

	req = httptest.NewRequest("GET", "/", nil)
	rr = httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != 405 {
		t.Errorf("GET: got %d, want 405", rr.Code)
	}
}

func TestReadOnly(t *testing.T) {
	handler := ReadOnly(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	safe := []string{"GET", "HEAD", "OPTIONS"}
	for _, m := range safe {
		req := httptest.NewRequest(m, "/", nil)
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		if rr.Code != 200 {
			t.Errorf("%s: got %d, want 200", m, rr.Code)
		}
	}

	unsafe := []string{"POST", "PUT", "DELETE", "PATCH"}
	for _, m := range unsafe {
		req := httptest.NewRequest(m, "/", nil)
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		if rr.Code != 405 {
			t.Errorf("%s: got %d, want 405", m, rr.Code)
		}
	}
}
