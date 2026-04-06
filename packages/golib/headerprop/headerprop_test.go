package headerprop

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestPropagator_Extract(t *testing.T) {
	p := New()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Request-ID", "req-123")
	req.Header.Set("X-Correlation-ID", "corr-456")
	req.Header.Set("Authorization", "Bearer secret") // Not in default list.

	headers := p.Extract(req)
	if headers["X-Request-ID"] != "req-123" {
		t.Error("should extract X-Request-ID")
	}
	if headers["X-Correlation-ID"] != "corr-456" {
		t.Error("should extract X-Correlation-ID")
	}
	if _, ok := headers["Authorization"]; ok {
		t.Error("should not extract non-propagated headers")
	}
}

func TestPropagator_Inject(t *testing.T) {
	p := New()
	req, _ := http.NewRequest(http.MethodGet, "http://example.com", nil)

	p.Inject(req, map[string]string{
		"X-Request-ID":     "req-123",
		"X-Correlation-ID": "corr-456",
	})

	if req.Header.Get("X-Request-ID") != "req-123" {
		t.Error("should inject X-Request-ID")
	}
}

func TestPropagator_InjectFromRequest(t *testing.T) {
	p := New()

	src := httptest.NewRequest(http.MethodGet, "/", nil)
	src.Header.Set("X-Request-ID", "from-src")
	src.Header.Set("X-Tenant-ID", "tenant-1")

	dst, _ := http.NewRequest(http.MethodGet, "http://downstream", nil)
	p.InjectFromRequest(dst, src)

	if dst.Header.Get("X-Request-ID") != "from-src" {
		t.Error("should propagate X-Request-ID")
	}
	if dst.Header.Get("X-Tenant-ID") != "tenant-1" {
		t.Error("should propagate X-Tenant-ID")
	}
}

func TestPropagator_Middleware(t *testing.T) {
	p := New()
	handler := p.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Request-ID", "echo-me")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Header().Get("X-Request-ID") != "echo-me" {
		t.Error("should echo propagated header in response")
	}
}

func TestPropagator_CustomHeaders(t *testing.T) {
	p := New("X-Custom-1", "X-Custom-2")

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Custom-1", "val1")
	req.Header.Set("X-Request-ID", "should-be-ignored")

	headers := p.Extract(req)
	if headers["X-Custom-1"] != "val1" {
		t.Error("should extract custom header")
	}
	if _, ok := headers["X-Request-ID"]; ok {
		t.Error("should not extract non-configured header")
	}
}

func TestPropagator_Count(t *testing.T) {
	p := New()
	if p.Count() != len(DefaultHeaders) {
		t.Errorf("count: got %d, want %d", p.Count(), len(DefaultHeaders))
	}
}

func TestPropagator_Headers(t *testing.T) {
	p := New("A", "B")
	h := p.Headers()
	if len(h) != 2 {
		t.Errorf("headers: got %d", len(h))
	}
	// Verify it's a copy.
	h[0] = "mutated"
	if p.Headers()[0] == "mutated" {
		t.Error("Headers() should return a copy")
	}
}

func TestRoundTripper(t *testing.T) {
	var capturedReq *http.Request
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedReq = r
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	p := New()
	src := httptest.NewRequest(http.MethodGet, "/", nil)
	src.Header.Set("X-Request-ID", "propagated")

	client := &http.Client{
		Transport: &RoundTripper{Propagator: p, Source: src},
	}

	resp, err := client.Get(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()

	if capturedReq.Header.Get("X-Request-ID") != "propagated" {
		t.Error("RoundTripper should propagate headers")
	}
}
