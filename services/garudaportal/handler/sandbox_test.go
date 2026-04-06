package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func newTestSandboxHandler(t *testing.T, backend http.Handler) (*SandboxHandler, *InMemorySandboxStore) {
	t.Helper()
	srv := httptest.NewServer(backend)
	t.Cleanup(srv.Close)
	store := NewInMemorySandboxStore()
	h := NewSandboxHandler(srv.Client(), store)
	return h, store
}

func TestExecuteRequest_SandboxKeySucceeds(t *testing.T) {
	backend := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Custom", "test-value")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"result":"ok"}`))
	})
	srv := httptest.NewServer(backend)
	defer srv.Close()

	store := NewInMemorySandboxStore()
	h := NewSandboxHandler(srv.Client(), store)

	body := `{"method":"GET","url":"` + srv.URL + `/api/v1/identity/verify","headers":{"Content-Type":"application/json"},"body":"","api_key":"gp_test_abc123"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/portal/sandbox/execute", strings.NewReader(body))
	req.Header.Set("X-User-ID", "user-1")
	w := httptest.NewRecorder()

	h.ExecuteRequest(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp executeResponse
	json.NewDecoder(w.Body).Decode(&resp)

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status_code 200, got %d", resp.StatusCode)
	}
	if resp.Body != `{"result":"ok"}` {
		t.Errorf("expected body {\"result\":\"ok\"}, got %s", resp.Body)
	}
	if resp.LatencyMs < 0 {
		t.Errorf("expected non-negative latency, got %d", resp.LatencyMs)
	}
	if resp.Headers["X-Custom"] != "test-value" {
		t.Errorf("expected header X-Custom=test-value, got %s", resp.Headers["X-Custom"])
	}
}

func TestExecuteRequest_ProductionKeyReturns403(t *testing.T) {
	h, _ := newTestSandboxHandler(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))

	body := `{"method":"GET","url":"http://example.com","api_key":"gp_live_abc123"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/portal/sandbox/execute", strings.NewReader(body))
	w := httptest.NewRecorder()

	h.ExecuteRequest(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", w.Code, w.Body.String())
	}
}

func TestExecuteRequest_MissingKeyReturns400(t *testing.T) {
	h, _ := newTestSandboxHandler(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))

	body := `{"method":"GET","url":"http://example.com"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/portal/sandbox/execute", strings.NewReader(body))
	w := httptest.NewRecorder()

	h.ExecuteRequest(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestExecuteRequest_RecordsHistory(t *testing.T) {
	backend := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	srv := httptest.NewServer(backend)
	defer srv.Close()

	store := NewInMemorySandboxStore()
	h := NewSandboxHandler(srv.Client(), store)

	body := `{"method":"POST","url":"` + srv.URL + `/api/v1/identity/register","api_key":"gp_test_xyz789"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/portal/sandbox/execute", strings.NewReader(body))
	req.Header.Set("X-User-ID", "user-42")
	w := httptest.NewRecorder()

	h.ExecuteRequest(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	history, err := store.ListByUser("user-42", 50)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(history) != 1 {
		t.Fatalf("expected 1 history entry, got %d", len(history))
	}
	if history[0].Method != "POST" {
		t.Errorf("expected POST, got %s", history[0].Method)
	}
	if history[0].UserID != "user-42" {
		t.Errorf("expected user-42, got %s", history[0].UserID)
	}
}

func TestListEndpoints_ReturnsEndpoints(t *testing.T) {
	store := NewInMemorySandboxStore()
	h := NewSandboxHandler(http.DefaultClient, store)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/portal/sandbox/endpoints", nil)
	w := httptest.NewRecorder()

	h.ListEndpoints(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp struct {
		Endpoints []EndpointInfo `json:"endpoints"`
	}
	json.NewDecoder(w.Body).Decode(&resp)

	if len(resp.Endpoints) == 0 {
		t.Error("expected at least one endpoint")
	}

	// Verify each endpoint has required fields.
	for _, ep := range resp.Endpoints {
		if ep.Method == "" || ep.Path == "" || ep.Description == "" {
			t.Errorf("endpoint missing fields: %+v", ep)
		}
	}
}

func TestGetRequestHistory_ReturnsUserRequests(t *testing.T) {
	store := NewInMemorySandboxStore()
	store.Save(SandboxRequest{ID: "r1", UserID: "user-1", Method: "GET", URL: "/test", Status: 200, LatencyMs: 10})
	store.Save(SandboxRequest{ID: "r2", UserID: "user-2", Method: "POST", URL: "/other", Status: 201, LatencyMs: 20})
	store.Save(SandboxRequest{ID: "r3", UserID: "user-1", Method: "PUT", URL: "/update", Status: 200, LatencyMs: 15})

	h := NewSandboxHandler(http.DefaultClient, store)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/portal/sandbox/history", nil)
	req.Header.Set("X-User-ID", "user-1")
	w := httptest.NewRecorder()

	h.GetRequestHistory(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp struct {
		Requests []SandboxRequest `json:"requests"`
	}
	json.NewDecoder(w.Body).Decode(&resp)

	if len(resp.Requests) != 2 {
		t.Fatalf("expected 2 requests for user-1, got %d", len(resp.Requests))
	}

	// Most recent first.
	if resp.Requests[0].ID != "r3" {
		t.Errorf("expected most recent request first, got %s", resp.Requests[0].ID)
	}
}

func TestGetRequestHistory_LimitedTo50(t *testing.T) {
	store := NewInMemorySandboxStore()

	// Insert 60 requests for the same user.
	for i := range 60 {
		store.Save(SandboxRequest{
			ID:     "r" + strings.Repeat("x", i%10+1),
			UserID: "user-1",
			Method: "GET",
			URL:    "/test",
			Status: 200,
		})
	}

	h := NewSandboxHandler(http.DefaultClient, store)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/portal/sandbox/history", nil)
	req.Header.Set("X-User-ID", "user-1")
	w := httptest.NewRecorder()

	h.GetRequestHistory(w, req)

	var resp struct {
		Requests []SandboxRequest `json:"requests"`
	}
	json.NewDecoder(w.Body).Decode(&resp)

	if len(resp.Requests) != 50 {
		t.Errorf("expected max 50 requests, got %d", len(resp.Requests))
	}
}
