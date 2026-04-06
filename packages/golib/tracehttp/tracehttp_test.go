package tracehttp

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestMiddleware_GeneratesTraceparent(t *testing.T) {
	handler := Middleware("test-service")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tp := r.Header.Get(HeaderTraceparent)
		if tp == "" {
			t.Error("should inject traceparent")
		}
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Header().Get(HeaderTraceparent) == "" {
		t.Error("should echo traceparent in response")
	}
	if w.Header().Get("X-Origin-Service") != "test-service" {
		t.Error("should set origin service")
	}
}

func TestMiddleware_PreservesExisting(t *testing.T) {
	existing := "00-abcdef0123456789abcdef0123456789-0123456789abcdef-01"
	handler := Middleware("svc")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get(HeaderTraceparent) != existing {
			t.Error("should preserve existing traceparent")
		}
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set(HeaderTraceparent, existing)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Header().Get(HeaderTraceparent) != existing {
		t.Error("should echo existing traceparent")
	}
}

func TestMiddleware_PropagatesTracestate(t *testing.T) {
	handler := Middleware("svc")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set(HeaderTraceparent, "00-abc-def-01")
	req.Header.Set(HeaderTracestate, "vendor=value")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Header().Get(HeaderTracestate) != "vendor=value" {
		t.Error("should propagate tracestate")
	}
}

func TestTransport_InjectsHeaders(t *testing.T) {
	var capturedReq *http.Request
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedReq = r
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	tr := NewTransport(http.DefaultTransport, Config{
		ServiceName:       "caller",
		GenerateIfMissing: true,
	})

	client := tr.Client()
	resp, err := client.Get(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()

	if capturedReq.Header.Get(HeaderTraceparent) == "" {
		t.Error("should inject traceparent")
	}
	if capturedReq.Header.Get("X-Origin-Service") != "caller" {
		t.Error("should inject origin service")
	}
}

func TestTransport_PreservesExisting(t *testing.T) {
	existing := "00-abc123-def456-01"
	var capturedReq *http.Request
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedReq = r
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	tr := NewTransport(http.DefaultTransport, Config{GenerateIfMissing: true})
	client := tr.Client()

	req, _ := http.NewRequest(http.MethodGet, server.URL, nil)
	req.Header.Set(HeaderTraceparent, existing)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()

	if capturedReq.Header.Get(HeaderTraceparent) != existing {
		t.Error("should preserve existing traceparent")
	}
}

func TestTransport_DefaultBase(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	tr := NewTransport(nil, Config{ServiceName: "svc"}) // nil base.
	client := tr.Client()
	resp, err := client.Get(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
}

func TestGenerateHex(t *testing.T) {
	h := generateHex(16)
	if len(h) != 32 {
		t.Errorf("hex length: got %d", len(h))
	}
	// Should be valid hex.
	for _, c := range h {
		if !strings.ContainsRune("0123456789abcdef", c) {
			t.Errorf("invalid hex char: %c", c)
		}
	}
}
