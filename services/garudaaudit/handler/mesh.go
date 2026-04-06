package handler

import (
	"encoding/json"
	"net/http"
	"sync"
	"time"
)

// DefaultServices defines the default service health endpoints.
var DefaultServices = map[string]string{
	"bff":          "http://localhost:4000/health",
	"identity":     "http://localhost:4001/health",
	"garudainfo":   "http://localhost:4003/health",
	"garudacorp":   "http://localhost:4006/health",
	"garudasign":   "http://localhost:4007/health",
	"garudaportal": "http://localhost:4009/health",
	"garudaaudit":  "http://localhost:4010/health",
	"garudanotify": "http://localhost:4011/health",
}

// MeshHandler provides platform-wide service mesh health.
type MeshHandler struct {
	services map[string]string // name -> health URL
	client   *http.Client
}

// NewMeshHandler creates a MeshHandler with the given service map and HTTP client.
func NewMeshHandler(services map[string]string, client *http.Client) *MeshHandler {
	if client == nil {
		client = &http.Client{Timeout: 5 * time.Second}
	}
	return &MeshHandler{
		services: services,
		client:   client,
	}
}

// serviceHealth represents the health status of a single service.
type serviceHealth struct {
	Name      string `json:"name"`
	URL       string `json:"url"`
	Status    string `json:"status"` // "healthy" or "unhealthy"
	LatencyMs int64  `json:"latency_ms"`
	Error     string `json:"error,omitempty"`
}

// meshHealthResponse is the response for GET /api/v1/audit/mesh/health.
type meshHealthResponse struct {
	Status         string          `json:"status"` // "healthy", "degraded", "critical"
	Services       []serviceHealth `json:"services"`
	HealthyCount   int             `json:"healthy_count"`
	UnhealthyCount int             `json:"unhealthy_count"`
	TotalCount     int             `json:"total_count"`
	CheckedAt      string          `json:"checked_at"`
}

// GetMeshHealth handles GET /api/v1/audit/mesh/health.
// Concurrently checks all service health endpoints.
func (h *MeshHandler) GetMeshHealth(w http.ResponseWriter, r *http.Request) {
	results := make([]serviceHealth, 0, len(h.services))
	var mu sync.Mutex
	var wg sync.WaitGroup

	for name, url := range h.services {
		wg.Add(1)
		go func(name, url string) {
			defer wg.Done()
			sh := h.checkService(name, url)
			mu.Lock()
			results = append(results, sh)
			mu.Unlock()
		}(name, url)
	}

	wg.Wait()

	healthy := 0
	unhealthy := 0
	for _, s := range results {
		if s.Status == "healthy" {
			healthy++
		} else {
			unhealthy++
		}
	}

	total := len(results)
	var overallStatus string
	switch {
	case unhealthy == 0:
		overallStatus = "healthy"
	case healthy == 0:
		overallStatus = "critical"
	default:
		overallStatus = "degraded"
	}

	resp := meshHealthResponse{
		Status:         overallStatus,
		Services:       results,
		HealthyCount:   healthy,
		UnhealthyCount: unhealthy,
		TotalCount:     total,
		CheckedAt:      time.Now().UTC().Format(time.RFC3339),
	}

	writeJSON(w, http.StatusOK, resp)
}

// checkService checks a single service health endpoint.
func (h *MeshHandler) checkService(name, url string) serviceHealth {
	start := time.Now()

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return serviceHealth{
			Name:      name,
			URL:       url,
			Status:    "unhealthy",
			LatencyMs: time.Since(start).Milliseconds(),
			Error:     err.Error(),
		}
	}

	resp, err := h.client.Do(req)
	latency := time.Since(start).Milliseconds()

	if err != nil {
		return serviceHealth{
			Name:      name,
			URL:       url,
			Status:    "unhealthy",
			LatencyMs: latency,
			Error:     err.Error(),
		}
	}
	defer resp.Body.Close()

	// Check that we got a 200 and the body contains status: ok.
	if resp.StatusCode != http.StatusOK {
		return serviceHealth{
			Name:      name,
			URL:       url,
			Status:    "unhealthy",
			LatencyMs: latency,
			Error:     "unexpected status code",
		}
	}

	var body map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return serviceHealth{
			Name:      name,
			URL:       url,
			Status:    "unhealthy",
			LatencyMs: latency,
			Error:     "invalid response body",
		}
	}

	if body["status"] != "ok" {
		return serviceHealth{
			Name:      name,
			URL:       url,
			Status:    "unhealthy",
			LatencyMs: latency,
			Error:     "status not ok",
		}
	}

	return serviceHealth{
		Name:      name,
		URL:       url,
		Status:    "healthy",
		LatencyMs: latency,
	}
}
