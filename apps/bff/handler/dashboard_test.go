package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestDashboard_ReturnsPlatformStatus(t *testing.T) {
	healthy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"status":"ok"}`)
	}))
	defer healthy.Close()

	agg := NewHealthAggregatorWithServices("v1.2.3", map[string]string{
		"identity": healthy.URL,
	})
	dh := NewDashboardHandlerWithStart(agg, "development", time.Now().Add(-10*time.Second))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/dashboard", nil)
	w := httptest.NewRecorder()
	dh.GetDashboard(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp DashboardResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode error: %v", err)
	}

	if resp.Platform.Status != "ok" {
		t.Errorf("platform status: got %q, want %q", resp.Platform.Status, "ok")
	}
	if resp.Platform.Version != "v1.2.3" {
		t.Errorf("platform version: got %q, want %q", resp.Platform.Version, "v1.2.3")
	}
	if resp.Platform.UptimeSeconds < 10 {
		t.Errorf("expected uptime >= 10s, got %f", resp.Platform.UptimeSeconds)
	}
}

func TestDashboard_IncludesServiceList(t *testing.T) {
	healthy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"status":"ok"}`)
	}))
	defer healthy.Close()

	unhealthy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer unhealthy.Close()

	agg := NewHealthAggregatorWithServices("v1", map[string]string{
		"svc-a": healthy.URL,
		"svc-b": unhealthy.URL,
	})
	dh := NewDashboardHandler(agg, "staging")

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/dashboard", nil)
	w := httptest.NewRecorder()
	dh.GetDashboard(w, req)

	var resp DashboardResponse
	json.NewDecoder(w.Body).Decode(&resp)

	if len(resp.Services) != 2 {
		t.Fatalf("expected 2 services, got %d", len(resp.Services))
	}

	// Verify each service has required fields
	for _, svc := range resp.Services {
		if svc.Name == "" {
			t.Error("service name should not be empty")
		}
		if svc.Status == "" {
			t.Error("service status should not be empty")
		}
		if svc.LatencyMs < 0 {
			t.Errorf("service %s: latency should be >= 0, got %f", svc.Name, svc.LatencyMs)
		}
	}
}

func TestDashboard_StatsCalculation(t *testing.T) {
	healthy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"status":"ok"}`)
	}))
	defer healthy.Close()

	unhealthy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer unhealthy.Close()

	agg := NewHealthAggregatorWithServices("v1", map[string]string{
		"healthy-1":  healthy.URL,
		"healthy-2":  healthy.URL,
		"unhealthy":  unhealthy.URL,
	})
	dh := NewDashboardHandler(agg, "production")

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/dashboard", nil)
	w := httptest.NewRecorder()
	dh.GetDashboard(w, req)

	var resp DashboardResponse
	json.NewDecoder(w.Body).Decode(&resp)

	if resp.Stats.TotalServices != 3 {
		t.Errorf("total_services: got %d, want 3", resp.Stats.TotalServices)
	}
	if resp.Stats.HealthyServices != 2 {
		t.Errorf("healthy_services: got %d, want 2", resp.Stats.HealthyServices)
	}
	// unhealthy services are not "degraded" — they are "unhealthy"
	if resp.Stats.DegradedServices != 0 {
		t.Errorf("degraded_services: got %d, want 0", resp.Stats.DegradedServices)
	}
	if resp.Stats.TotalEndpoints != 3 {
		t.Errorf("total_endpoints: got %d, want 3", resp.Stats.TotalEndpoints)
	}
	if resp.Stats.AvgLatencyMs < 0 {
		t.Errorf("avg_latency_ms should be >= 0, got %f", resp.Stats.AvgLatencyMs)
	}
}

func TestDashboard_IncludesEnvironment(t *testing.T) {
	healthy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"status":"ok"}`)
	}))
	defer healthy.Close()

	tests := []string{"development", "staging", "production"}
	for _, env := range tests {
		t.Run(env, func(t *testing.T) {
			agg := NewHealthAggregatorWithServices("v1", map[string]string{
				"svc": healthy.URL,
			})
			dh := NewDashboardHandler(agg, env)

			req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/dashboard", nil)
			w := httptest.NewRecorder()
			dh.GetDashboard(w, req)

			var resp DashboardResponse
			json.NewDecoder(w.Body).Decode(&resp)

			if resp.Environment != env {
				t.Errorf("environment: got %q, want %q", resp.Environment, env)
			}
			if resp.CheckedAt == "" {
				t.Error("checked_at should not be empty")
			}
		})
	}
}
