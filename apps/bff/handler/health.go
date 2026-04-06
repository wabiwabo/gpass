package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
)

// ServiceHealth represents the health status of a downstream service.
type ServiceHealth struct {
	Name    string `json:"name"`
	URL     string `json:"url"`
	Status  string `json:"status"`  // ok, degraded, unhealthy
	Latency string `json:"latency"` // e.g. "12ms"
	Error   string `json:"error,omitempty"`
}

// AggregatedHealth is the combined health status of the platform.
type AggregatedHealth struct {
	Status   string          `json:"status"` // ok, degraded, unhealthy
	Version  string          `json:"version"`
	Services []ServiceHealth `json:"services"`
	CheckedAt string         `json:"checked_at"`
}

// HealthAggregator checks health of all downstream services concurrently.
type HealthAggregator struct {
	services []serviceEndpoint
	client   *http.Client
	version  string
}

type serviceEndpoint struct {
	name string
	url  string
}

// NewHealthAggregator creates a health aggregator for all platform services.
func NewHealthAggregator(version string) *HealthAggregator {
	return &HealthAggregator{
		services: []serviceEndpoint{
			{name: "identity", url: "http://localhost:4001/health"},
			{name: "garudainfo", url: "http://localhost:4003/health"},
			{name: "garudacorp", url: "http://localhost:4006/health"},
			{name: "garudasign", url: "http://localhost:4007/health"},
			{name: "garudaportal", url: "http://localhost:4009/health"},
			{name: "garudaaudit", url: "http://localhost:4010/health"},
			{name: "garudanotify", url: "http://localhost:4011/health"},
		},
		client: &http.Client{
			Timeout: 5 * time.Second,
		},
		version: version,
	}
}

// NewHealthAggregatorWithServices creates a health aggregator with custom service URLs.
func NewHealthAggregatorWithServices(version string, services map[string]string) *HealthAggregator {
	endpoints := make([]serviceEndpoint, 0, len(services))
	for name, url := range services {
		endpoints = append(endpoints, serviceEndpoint{name: name, url: url})
	}
	return &HealthAggregator{
		services: endpoints,
		client:   &http.Client{Timeout: 5 * time.Second},
		version:  version,
	}
}

// CheckAll checks health of all services concurrently.
func (h *HealthAggregator) CheckAll(ctx context.Context) *AggregatedHealth {
	results := make([]ServiceHealth, len(h.services))
	var wg sync.WaitGroup

	for i, svc := range h.services {
		wg.Add(1)
		go func(idx int, s serviceEndpoint) {
			defer wg.Done()
			results[idx] = h.checkService(ctx, s)
		}(i, svc)
	}

	wg.Wait()

	// Determine overall status
	unhealthyCount := 0
	for _, r := range results {
		if r.Status == "unhealthy" {
			unhealthyCount++
		}
	}

	overallStatus := "ok"
	if unhealthyCount > 0 && unhealthyCount < len(results) {
		overallStatus = "degraded"
	} else if unhealthyCount == len(results) {
		overallStatus = "unhealthy"
	}

	return &AggregatedHealth{
		Status:    overallStatus,
		Version:   h.version,
		Services:  results,
		CheckedAt: time.Now().UTC().Format(time.RFC3339),
	}
}

func (h *HealthAggregator) checkService(ctx context.Context, svc serviceEndpoint) ServiceHealth {
	start := time.Now()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, svc.url, nil)
	if err != nil {
		return ServiceHealth{
			Name:    svc.name,
			URL:     svc.url,
			Status:  "unhealthy",
			Latency: time.Since(start).String(),
			Error:   fmt.Sprintf("create request: %v", err),
		}
	}

	resp, err := h.client.Do(req)
	latency := time.Since(start)

	if err != nil {
		return ServiceHealth{
			Name:    svc.name,
			URL:     svc.url,
			Status:  "unhealthy",
			Latency: latency.String(),
			Error:   fmt.Sprintf("request failed: %v", err),
		}
	}
	defer resp.Body.Close()

	status := "ok"
	if resp.StatusCode != http.StatusOK {
		status = "unhealthy"
	}

	return ServiceHealth{
		Name:    svc.name,
		URL:     svc.url,
		Status:  status,
		Latency: latency.String(),
	}
}

// ServeHTTP handles GET /health/all — aggregated health check.
func (h *HealthAggregator) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	result := h.CheckAll(r.Context())

	status := http.StatusOK
	if result.Status == "degraded" {
		status = http.StatusOK // still 200 but status field shows degraded
	} else if result.Status == "unhealthy" {
		status = http.StatusServiceUnavailable
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(result)
}
