package handler

import (
	"encoding/json"
	"math"
	"net/http"
	"time"
)

// DashboardResponse is the JSON structure returned by GET /api/v1/admin/dashboard.
type DashboardResponse struct {
	Platform    PlatformInfo     `json:"platform"`
	Services    []DashboardSvc   `json:"services"`
	Stats       DashboardStats   `json:"stats"`
	Environment string           `json:"environment"`
	CheckedAt   string           `json:"checked_at"`
}

// PlatformInfo describes the overall platform status.
type PlatformInfo struct {
	Status        string  `json:"status"`
	Version       string  `json:"version"`
	UptimeSeconds float64 `json:"uptime_seconds"`
}

// DashboardSvc is a per-service entry in the dashboard.
type DashboardSvc struct {
	Name      string  `json:"name"`
	Status    string  `json:"status"`
	LatencyMs float64 `json:"latency_ms"`
	Version   string  `json:"version"`
}

// DashboardStats contains aggregate statistics about the platform.
type DashboardStats struct {
	TotalServices    int     `json:"total_services"`
	HealthyServices  int     `json:"healthy_services"`
	DegradedServices int     `json:"degraded_services"`
	TotalEndpoints   int     `json:"total_endpoints"`
	AvgLatencyMs     float64 `json:"avg_latency_ms"`
}

// DashboardHandler provides platform-wide dashboard data.
type DashboardHandler struct {
	healthAgg   *HealthAggregator
	environment string
	startTime   time.Time
}

// NewDashboardHandler creates a new DashboardHandler.
func NewDashboardHandler(healthAgg *HealthAggregator, environment string) *DashboardHandler {
	return &DashboardHandler{
		healthAgg:   healthAgg,
		environment: environment,
		startTime:   time.Now(),
	}
}

// NewDashboardHandlerWithStart creates a DashboardHandler with a custom start time (for testing).
func NewDashboardHandlerWithStart(healthAgg *HealthAggregator, environment string, startTime time.Time) *DashboardHandler {
	return &DashboardHandler{
		healthAgg:   healthAgg,
		environment: environment,
		startTime:   startTime,
	}
}

// GetDashboard handles GET /api/v1/admin/dashboard.
func (h *DashboardHandler) GetDashboard(w http.ResponseWriter, r *http.Request) {
	agg := h.healthAgg.CheckAll(r.Context())

	services := make([]DashboardSvc, len(agg.Services))
	var totalLatencyMs float64
	healthyCount := 0
	degradedCount := 0

	for i, svc := range agg.Services {
		latency, _ := time.ParseDuration(svc.Latency)
		latencyMs := float64(latency.Microseconds()) / 1000.0

		services[i] = DashboardSvc{
			Name:      svc.Name,
			Status:    svc.Status,
			LatencyMs: math.Round(latencyMs*100) / 100,
			Version:   agg.Version, // service versions come from health check
		}

		totalLatencyMs += latencyMs

		switch svc.Status {
		case "ok":
			healthyCount++
		case "degraded":
			degradedCount++
		}
	}

	avgLatency := 0.0
	if len(services) > 0 {
		avgLatency = math.Round(totalLatencyMs/float64(len(services))*100) / 100
	}

	resp := DashboardResponse{
		Platform: PlatformInfo{
			Status:        agg.Status,
			Version:       agg.Version,
			UptimeSeconds: math.Round(time.Since(h.startTime).Seconds()*100) / 100,
		},
		Services: services,
		Stats: DashboardStats{
			TotalServices:    len(services),
			HealthyServices:  healthyCount,
			DegradedServices: degradedCount,
			TotalEndpoints:   len(services), // each service exposes a health endpoint
			AvgLatencyMs:     avgLatency,
		},
		Environment: h.environment,
		CheckedAt:   agg.CheckedAt,
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(resp)
}
