package correlation

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
)

func TestGenerateID_Uniqueness(t *testing.T) {
	seen := make(map[string]bool, 1000)
	for i := 0; i < 1000; i++ {
		id := GenerateID()
		if len(id) != 16 {
			t.Fatalf("expected 16 hex chars, got %d: %s", len(id), id)
		}
		if seen[id] {
			t.Fatalf("duplicate ID: %s", id)
		}
		seen[id] = true
	}
}

func TestGenerateID_HexFormat(t *testing.T) {
	id := GenerateID()
	for _, c := range id {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			t.Fatalf("non-hex character in ID: %c", c)
		}
	}
}

func TestMiddleware_GeneratesCorrelationID(t *testing.T) {
	var capturedInfo Info
	handler := Middleware("test-service")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedInfo, _ = FromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if capturedInfo.CorrelationID == "" {
		t.Error("correlation ID should be generated")
	}
	if capturedInfo.RequestID == "" {
		t.Error("request ID should be generated")
	}
	if w.Header().Get(HeaderCorrelationID) == "" {
		t.Error("correlation ID should be in response")
	}
	if w.Header().Get(HeaderRequestID) == "" {
		t.Error("request ID should be in response")
	}
}

func TestMiddleware_PreservesExistingCorrelationID(t *testing.T) {
	existingID := "existing-correlation-id"
	var capturedInfo Info
	handler := Middleware("test-service")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedInfo, _ = FromContext(r.Context())
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set(HeaderCorrelationID, existingID)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if capturedInfo.CorrelationID != existingID {
		t.Errorf("should preserve existing ID: got %q", capturedInfo.CorrelationID)
	}
}

func TestMiddleware_PropagatesCausationID(t *testing.T) {
	var capturedInfo Info
	handler := Middleware("downstream")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedInfo, _ = FromContext(r.Context())
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set(HeaderCorrelationID, "corr-123")
	req.Header.Set(HeaderCausationID, "cause-456")
	req.Header.Set(HeaderRequestID, "req-789")
	req.Header.Set(HeaderOriginService, "upstream")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if capturedInfo.CausationID != "cause-456" {
		t.Errorf("causation ID: got %q", capturedInfo.CausationID)
	}
	if capturedInfo.OriginService != "upstream" {
		t.Errorf("origin: got %q", capturedInfo.OriginService)
	}
	if capturedInfo.Depth != 1 {
		t.Errorf("depth: got %d, want 1", capturedInfo.Depth)
	}
}

func TestFromContext_Missing(t *testing.T) {
	_, ok := FromContext(context.Background())
	if ok {
		t.Error("should return false for empty context")
	}
}

func TestWithInfo_RoundTrip(t *testing.T) {
	info := Info{
		CorrelationID: "corr-1",
		RequestID:     "req-1",
		OriginService: "svc-a",
	}
	ctx := WithInfo(context.Background(), info)
	got, ok := FromContext(ctx)
	if !ok {
		t.Fatal("should find info")
	}
	if got.CorrelationID != "corr-1" {
		t.Errorf("correlation ID: got %q", got.CorrelationID)
	}
}

func TestPropagate_WithContext(t *testing.T) {
	info := Info{
		CorrelationID: "parent-corr",
		RequestID:     "parent-req",
	}
	ctx := WithInfo(context.Background(), info)

	headers := Propagate(ctx, "child-service")

	if headers.Get(HeaderCorrelationID) != "parent-corr" {
		t.Errorf("should propagate correlation ID: got %q", headers.Get(HeaderCorrelationID))
	}
	if headers.Get(HeaderCausationID) != "parent-req" {
		t.Errorf("causation should be parent's request ID: got %q", headers.Get(HeaderCausationID))
	}
	if headers.Get(HeaderRequestID) == "" || headers.Get(HeaderRequestID) == "parent-req" {
		t.Error("should generate new request ID for downstream")
	}
	if headers.Get(HeaderOriginService) != "child-service" {
		t.Errorf("origin should be child service: got %q", headers.Get(HeaderOriginService))
	}
}

func TestPropagate_WithoutContext(t *testing.T) {
	headers := Propagate(context.Background(), "first-service")

	if headers.Get(HeaderCorrelationID) == "" {
		t.Error("should generate new correlation ID")
	}
	if headers.Get(HeaderRequestID) == "" {
		t.Error("should generate new request ID")
	}
	if headers.Get(HeaderOriginService) != "first-service" {
		t.Errorf("origin: got %q", headers.Get(HeaderOriginService))
	}
}

func TestRoundTripper_InjectsHeaders(t *testing.T) {
	var capturedReq *http.Request
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedReq = r
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	info := Info{
		CorrelationID: "rt-corr",
		RequestID:     "rt-req",
	}
	ctx := WithInfo(context.Background(), info)

	client := &http.Client{
		Transport: &RoundTripper{
			Base:        http.DefaultTransport,
			ServiceName: "caller-svc",
		},
	}

	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, server.URL, nil)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	resp.Body.Close()

	if capturedReq.Header.Get(HeaderCorrelationID) != "rt-corr" {
		t.Errorf("correlation ID not propagated: got %q", capturedReq.Header.Get(HeaderCorrelationID))
	}
	if capturedReq.Header.Get(HeaderCausationID) != "rt-req" {
		t.Errorf("causation ID not set: got %q", capturedReq.Header.Get(HeaderCausationID))
	}
	if capturedReq.Header.Get(HeaderOriginService) != "caller-svc" {
		t.Errorf("origin not set: got %q", capturedReq.Header.Get(HeaderOriginService))
	}
}

func TestRoundTripper_DefaultTransport(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := &http.Client{
		Transport: &RoundTripper{ServiceName: "svc"},
	}
	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, server.URL, nil)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
}

func TestChain_AddAndList(t *testing.T) {
	c := NewChain()
	id1 := c.AddHop("svc-a", "req-1")
	id2 := c.AddHop("svc-b", "req-2")

	if id1 != 1 || id2 != 2 {
		t.Errorf("IDs: got %d, %d", id1, id2)
	}
	if c.Len() != 2 {
		t.Errorf("len: got %d", c.Len())
	}

	hops := c.Hops()
	if len(hops) != 2 {
		t.Fatalf("hops: got %d", len(hops))
	}
	if hops[0].Service != "svc-a" || hops[1].Service != "svc-b" {
		t.Errorf("services: got %q, %q", hops[0].Service, hops[1].Service)
	}
}

func TestChain_ConcurrentSafety(t *testing.T) {
	c := NewChain()
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			c.AddHop("svc", GenerateID())
		}(i)
	}
	wg.Wait()

	if c.Len() != 100 {
		t.Errorf("concurrent adds: got %d, want 100", c.Len())
	}
}

func TestChain_HopsIsolation(t *testing.T) {
	c := NewChain()
	c.AddHop("svc-a", "req-1")

	hops := c.Hops()
	hops[0].Service = "mutated" // Mutate the copy.

	original := c.Hops()
	if original[0].Service != "svc-a" {
		t.Error("Hops should return a copy, not a reference")
	}
}

func TestMiddleware_TimestampSet(t *testing.T) {
	var capturedInfo Info
	handler := Middleware("svc")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedInfo, _ = FromContext(r.Context())
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if capturedInfo.Timestamp.IsZero() {
		t.Error("timestamp should be set")
	}
}
