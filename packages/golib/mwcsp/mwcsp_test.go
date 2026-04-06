package mwcsp

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAPIPolicyString(t *testing.T) {
	p := APIPolicy()
	s := p.String()
	if !strings.Contains(s, "default-src 'none'") {
		t.Errorf("should contain default-src 'none': %s", s)
	}
	if !strings.Contains(s, "frame-src 'none'") {
		t.Errorf("should contain frame-src 'none': %s", s)
	}
}

func TestWebPolicyString(t *testing.T) {
	p := WebPolicy()
	s := p.String()
	if !strings.Contains(s, "default-src 'self'") {
		t.Errorf("should contain default-src 'self': %s", s)
	}
	if !strings.Contains(s, "script-src 'self'") {
		t.Errorf("should contain script-src: %s", s)
	}
	if !strings.Contains(s, "object-src 'none'") {
		t.Errorf("should contain object-src 'none': %s", s)
	}
}

func TestPolicyStringWithReportURI(t *testing.T) {
	p := APIPolicy()
	p.ReportURI = "/csp-report"
	s := p.String()
	if !strings.Contains(s, "report-uri /csp-report") {
		t.Errorf("should contain report-uri: %s", s)
	}
}

func TestAPI(t *testing.T) {
	handler := API(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	csp := rr.Header().Get("Content-Security-Policy")
	if csp == "" {
		t.Fatal("CSP header should be set")
	}
	if !strings.Contains(csp, "'none'") {
		t.Errorf("API CSP should be strict: %s", csp)
	}
}

func TestWeb(t *testing.T) {
	handler := Web(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	csp := rr.Header().Get("Content-Security-Policy")
	if !strings.Contains(csp, "'self'") {
		t.Errorf("Web CSP should allow self: %s", csp)
	}
}

func TestMiddlewarePassthrough(t *testing.T) {
	handler := API(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Test", "yes")
		w.WriteHeader(http.StatusCreated)
	}))

	req := httptest.NewRequest("GET", "/", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Errorf("status: got %d", rr.Code)
	}
	if rr.Header().Get("X-Test") != "yes" {
		t.Error("custom header should pass through")
	}
}

func TestEmptyPolicy(t *testing.T) {
	p := Policy{}
	s := p.String()
	if s != "" {
		t.Errorf("empty policy should produce empty string, got %q", s)
	}
}

func TestCustomPolicy(t *testing.T) {
	p := Policy{
		DefaultSrc: []string{"'self'"},
		ScriptSrc:  []string{"'self'", "https://cdn.example.com"},
		ImgSrc:     []string{"*"},
	}
	s := p.String()
	if !strings.Contains(s, "https://cdn.example.com") {
		t.Errorf("should contain CDN: %s", s)
	}
	if !strings.Contains(s, "img-src *") {
		t.Errorf("should contain img-src *: %s", s)
	}
}
