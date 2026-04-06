package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

// newMockService creates a test server that returns the given status and body.
func newMockService(status int, body string) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		fmt.Fprint(w, body)
	}))
}

func parseMeshResponse(t *testing.T, w *httptest.ResponseRecorder) meshHealthResponse {
	t.Helper()
	var resp meshHealthResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	return resp
}

func TestMeshHealth_AllHealthy(t *testing.T) {
	s1 := newMockService(http.StatusOK, `{"status":"ok"}`)
	defer s1.Close()
	s2 := newMockService(http.StatusOK, `{"status":"ok"}`)
	defer s2.Close()
	s3 := newMockService(http.StatusOK, `{"status":"ok"}`)
	defer s3.Close()

	services := map[string]string{
		"svc-a": s1.URL,
		"svc-b": s2.URL,
		"svc-c": s3.URL,
	}

	h := NewMeshHandler(services, s1.Client())
	req := httptest.NewRequest(http.MethodGet, "/api/v1/audit/mesh/health", nil)
	w := httptest.NewRecorder()
	h.GetMeshHealth(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	resp := parseMeshResponse(t, w)
	if resp.Status != "healthy" {
		t.Errorf("expected status healthy, got %s", resp.Status)
	}
	if resp.HealthyCount != 3 {
		t.Errorf("expected 3 healthy, got %d", resp.HealthyCount)
	}
	if resp.UnhealthyCount != 0 {
		t.Errorf("expected 0 unhealthy, got %d", resp.UnhealthyCount)
	}
	if resp.TotalCount != 3 {
		t.Errorf("expected 3 total, got %d", resp.TotalCount)
	}
}

func TestMeshHealth_SomeUnhealthy(t *testing.T) {
	healthy := newMockService(http.StatusOK, `{"status":"ok"}`)
	defer healthy.Close()
	unhealthy := newMockService(http.StatusServiceUnavailable, `{"status":"error"}`)
	defer unhealthy.Close()

	services := map[string]string{
		"svc-a": healthy.URL,
		"svc-b": unhealthy.URL,
	}

	h := NewMeshHandler(services, healthy.Client())
	req := httptest.NewRequest(http.MethodGet, "/api/v1/audit/mesh/health", nil)
	w := httptest.NewRecorder()
	h.GetMeshHealth(w, req)

	resp := parseMeshResponse(t, w)
	if resp.Status != "degraded" {
		t.Errorf("expected status degraded, got %s", resp.Status)
	}
	if resp.HealthyCount != 1 {
		t.Errorf("expected 1 healthy, got %d", resp.HealthyCount)
	}
	if resp.UnhealthyCount != 1 {
		t.Errorf("expected 1 unhealthy, got %d", resp.UnhealthyCount)
	}
}

func TestMeshHealth_AllUnhealthy(t *testing.T) {
	u1 := newMockService(http.StatusServiceUnavailable, `{"status":"error"}`)
	defer u1.Close()
	u2 := newMockService(http.StatusServiceUnavailable, `{"status":"error"}`)
	defer u2.Close()

	services := map[string]string{
		"svc-a": u1.URL,
		"svc-b": u2.URL,
	}

	h := NewMeshHandler(services, u1.Client())
	req := httptest.NewRequest(http.MethodGet, "/api/v1/audit/mesh/health", nil)
	w := httptest.NewRecorder()
	h.GetMeshHealth(w, req)

	resp := parseMeshResponse(t, w)
	if resp.Status != "critical" {
		t.Errorf("expected status critical, got %s", resp.Status)
	}
	if resp.HealthyCount != 0 {
		t.Errorf("expected 0 healthy, got %d", resp.HealthyCount)
	}
}

func TestMeshHealth_ServiceDown(t *testing.T) {
	healthy := newMockService(http.StatusOK, `{"status":"ok"}`)
	defer healthy.Close()

	services := map[string]string{
		"svc-up":   healthy.URL,
		"svc-down": "http://127.0.0.1:1", // port 1 — nothing listening
	}

	h := NewMeshHandler(services, nil)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/audit/mesh/health", nil)
	w := httptest.NewRecorder()
	h.GetMeshHealth(w, req)

	resp := parseMeshResponse(t, w)
	if resp.Status != "degraded" {
		t.Errorf("expected status degraded, got %s", resp.Status)
	}

	// Find the down service and check for error message.
	for _, svc := range resp.Services {
		if svc.Name == "svc-down" {
			if svc.Status != "unhealthy" {
				t.Errorf("expected svc-down to be unhealthy, got %s", svc.Status)
			}
			if svc.Error == "" {
				t.Error("expected error message for down service")
			}
			return
		}
	}
	t.Error("svc-down not found in response")
}

func TestMeshHealth_CountsAreCorrect(t *testing.T) {
	h1 := newMockService(http.StatusOK, `{"status":"ok"}`)
	defer h1.Close()
	h2 := newMockService(http.StatusOK, `{"status":"ok"}`)
	defer h2.Close()
	u1 := newMockService(http.StatusInternalServerError, `{"error":"fail"}`)
	defer u1.Close()

	services := map[string]string{
		"a": h1.URL,
		"b": h2.URL,
		"c": u1.URL,
	}

	h := NewMeshHandler(services, h1.Client())
	req := httptest.NewRequest(http.MethodGet, "/api/v1/audit/mesh/health", nil)
	w := httptest.NewRecorder()
	h.GetMeshHealth(w, req)

	resp := parseMeshResponse(t, w)
	if resp.HealthyCount != 2 {
		t.Errorf("expected 2 healthy, got %d", resp.HealthyCount)
	}
	if resp.UnhealthyCount != 1 {
		t.Errorf("expected 1 unhealthy, got %d", resp.UnhealthyCount)
	}
	if resp.TotalCount != 3 {
		t.Errorf("expected 3 total, got %d", resp.TotalCount)
	}
}

func TestMeshHealth_ResponseIncludesLatency(t *testing.T) {
	s := newMockService(http.StatusOK, `{"status":"ok"}`)
	defer s.Close()

	services := map[string]string{
		"svc": s.URL,
	}

	h := NewMeshHandler(services, s.Client())
	req := httptest.NewRequest(http.MethodGet, "/api/v1/audit/mesh/health", nil)
	w := httptest.NewRecorder()
	h.GetMeshHealth(w, req)

	resp := parseMeshResponse(t, w)
	if len(resp.Services) != 1 {
		t.Fatalf("expected 1 service, got %d", len(resp.Services))
	}

	// Latency should be non-negative (it was measured).
	if resp.Services[0].LatencyMs < 0 {
		t.Errorf("expected non-negative latency, got %d", resp.Services[0].LatencyMs)
	}
}

func TestMeshHealth_CheckedAtPresent(t *testing.T) {
	s := newMockService(http.StatusOK, `{"status":"ok"}`)
	defer s.Close()

	services := map[string]string{"svc": s.URL}

	h := NewMeshHandler(services, s.Client())
	req := httptest.NewRequest(http.MethodGet, "/api/v1/audit/mesh/health", nil)
	w := httptest.NewRecorder()
	h.GetMeshHealth(w, req)

	resp := parseMeshResponse(t, w)
	if resp.CheckedAt == "" {
		t.Error("expected checked_at to be present")
	}
}

func TestMeshHealth_StatusNotOk(t *testing.T) {
	// Returns 200 but status is not "ok".
	s := newMockService(http.StatusOK, `{"status":"degraded"}`)
	defer s.Close()

	services := map[string]string{"svc": s.URL}

	h := NewMeshHandler(services, s.Client())
	req := httptest.NewRequest(http.MethodGet, "/api/v1/audit/mesh/health", nil)
	w := httptest.NewRecorder()
	h.GetMeshHealth(w, req)

	resp := parseMeshResponse(t, w)
	if resp.Status != "critical" {
		t.Errorf("expected critical, got %s", resp.Status)
	}
	if resp.Services[0].Error != "status not ok" {
		t.Errorf("expected 'status not ok' error, got %q", resp.Services[0].Error)
	}
}

func TestNewMeshHandler_NilClient(t *testing.T) {
	h := NewMeshHandler(map[string]string{}, nil)
	if h.client == nil {
		t.Error("expected default client to be created")
	}
}

func TestDefaultServices_HasExpectedEntries(t *testing.T) {
	expected := []string{"bff", "identity", "garudainfo", "garudacorp", "garudasign", "garudaportal", "garudaaudit", "garudanotify"}
	for _, name := range expected {
		if _, ok := DefaultServices[name]; !ok {
			t.Errorf("expected default service %q", name)
		}
	}
}
