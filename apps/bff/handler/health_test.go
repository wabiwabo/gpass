package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHealthAggregator_AllHealthy(t *testing.T) {
	// Create mock healthy services
	healthy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"status":"ok"}`)
	}))
	defer healthy.Close()

	agg := NewHealthAggregatorWithServices("test-v1", map[string]string{
		"svc-a": healthy.URL,
		"svc-b": healthy.URL,
	})

	result := agg.CheckAll(context.Background())

	if result.Status != "ok" {
		t.Errorf("expected ok, got %s", result.Status)
	}
	if result.Version != "test-v1" {
		t.Errorf("expected test-v1, got %s", result.Version)
	}
	if len(result.Services) != 2 {
		t.Errorf("expected 2 services, got %d", len(result.Services))
	}
	for _, svc := range result.Services {
		if svc.Status != "ok" {
			t.Errorf("service %s: expected ok, got %s", svc.Name, svc.Status)
		}
	}
}

func TestHealthAggregator_SomeDegraded(t *testing.T) {
	healthy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"status":"ok"}`)
	}))
	defer healthy.Close()

	unhealthy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, `{"status":"error"}`)
	}))
	defer unhealthy.Close()

	agg := NewHealthAggregatorWithServices("v1", map[string]string{
		"healthy":   healthy.URL,
		"unhealthy": unhealthy.URL,
	})

	result := agg.CheckAll(context.Background())

	if result.Status != "degraded" {
		t.Errorf("expected degraded, got %s", result.Status)
	}
}

func TestHealthAggregator_AllUnhealthy(t *testing.T) {
	unhealthy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer unhealthy.Close()

	agg := NewHealthAggregatorWithServices("v1", map[string]string{
		"svc-a": unhealthy.URL,
		"svc-b": unhealthy.URL,
	})

	result := agg.CheckAll(context.Background())

	if result.Status != "unhealthy" {
		t.Errorf("expected unhealthy, got %s", result.Status)
	}
}

func TestHealthAggregator_ServiceDown(t *testing.T) {
	agg := NewHealthAggregatorWithServices("v1", map[string]string{
		"down": "http://localhost:59999/health", // nothing running here
	})

	result := agg.CheckAll(context.Background())

	if result.Status != "unhealthy" {
		t.Errorf("expected unhealthy, got %s", result.Status)
	}
	if len(result.Services) != 1 {
		t.Fatalf("expected 1 service, got %d", len(result.Services))
	}
	if result.Services[0].Error == "" {
		t.Error("expected error message for down service")
	}
}

func TestHealthAggregator_ServeHTTP_OK(t *testing.T) {
	healthy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"status":"ok"}`)
	}))
	defer healthy.Close()

	agg := NewHealthAggregatorWithServices("v1", map[string]string{
		"svc": healthy.URL,
	})

	req := httptest.NewRequest(http.MethodGet, "/health/all", nil)
	w := httptest.NewRecorder()
	agg.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	var result AggregatedHealth
	json.NewDecoder(w.Body).Decode(&result)
	if result.Status != "ok" {
		t.Errorf("expected ok, got %s", result.Status)
	}
	if w.Header().Get("Cache-Control") != "no-store" {
		t.Error("expected Cache-Control: no-store")
	}
}

func TestHealthAggregator_ServeHTTP_Unhealthy(t *testing.T) {
	agg := NewHealthAggregatorWithServices("v1", map[string]string{
		"down": "http://localhost:59999/health",
	})

	req := httptest.NewRequest(http.MethodGet, "/health/all", nil)
	w := httptest.NewRecorder()
	agg.ServeHTTP(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503, got %d", w.Code)
	}
}

func TestHealthAggregator_ConcurrentChecks(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		fmt.Fprint(w, `{"status":"ok"}`)
	}))
	defer server.Close()

	services := make(map[string]string)
	for i := 0; i < 5; i++ {
		services[fmt.Sprintf("svc-%d", i)] = server.URL
	}

	agg := NewHealthAggregatorWithServices("v1", services)
	result := agg.CheckAll(context.Background())

	if result.Status != "ok" {
		t.Errorf("expected ok, got %s", result.Status)
	}
	if len(result.Services) != 5 {
		t.Errorf("expected 5 services, got %d", len(result.Services))
	}
}
