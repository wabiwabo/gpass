package mwpermpolicy

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDefaultPolicy(t *testing.T) {
	p := DefaultPolicy()
	s := p.String()

	for _, feature := range []string{"camera", "microphone", "geolocation", "payment", "usb", "bluetooth"} {
		if !strings.Contains(s, feature+"=()") {
			t.Errorf("should disable %s: %s", feature, s)
		}
	}
}

func TestSimple(t *testing.T) {
	handler := Simple(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	pp := rr.Header().Get("Permissions-Policy")
	if pp == "" {
		t.Fatal("Permissions-Policy header should be set")
	}
	if !strings.Contains(pp, "camera") {
		t.Errorf("should contain camera: %s", pp)
	}
}

func TestCustomPolicy(t *testing.T) {
	p := Policy{
		Camera:      "(self)",
		Geolocation: "(self https://maps.example.com)",
	}
	handler := Middleware(p)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	pp := rr.Header().Get("Permissions-Policy")
	if !strings.Contains(pp, "camera=(self)") {
		t.Errorf("should contain camera=(self): %s", pp)
	}
	if !strings.Contains(pp, "geolocation=(self https://maps.example.com)") {
		t.Errorf("should contain geolocation: %s", pp)
	}
}

func TestEmptyPolicy(t *testing.T) {
	p := Policy{}
	if p.String() != "" {
		t.Errorf("empty policy should produce empty string, got %q", p.String())
	}
}

func TestPassthrough(t *testing.T) {
	handler := Simple(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte("body"))
	}))

	req := httptest.NewRequest("GET", "/", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Errorf("status: %d", rr.Code)
	}
	if rr.Body.String() != "body" {
		t.Errorf("body: %q", rr.Body.String())
	}
}

func TestDefaultPolicyAllFields(t *testing.T) {
	p := DefaultPolicy()
	if p.Camera == "" {
		t.Error("Camera should be set")
	}
	if p.Microphone == "" {
		t.Error("Microphone should be set")
	}
	if p.Geolocation == "" {
		t.Error("Geolocation should be set")
	}
	if p.Payment == "" {
		t.Error("Payment should be set")
	}
}
