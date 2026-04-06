package handler

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// SandboxRequest represents a recorded sandbox API request.
type SandboxRequest struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	Method    string    `json:"method"`
	URL       string    `json:"url"`
	Status    int       `json:"status_code"`
	LatencyMs int64     `json:"latency_ms"`
	CreatedAt time.Time `json:"created_at"`
}

// SandboxStore persists sandbox request history.
type SandboxStore interface {
	Save(req SandboxRequest) error
	ListByUser(userID string, limit int) ([]SandboxRequest, error)
}

// InMemorySandboxStore is an in-memory implementation of SandboxStore.
type InMemorySandboxStore struct {
	mu       sync.Mutex
	requests []SandboxRequest
}

// NewInMemorySandboxStore creates a new in-memory sandbox store.
func NewInMemorySandboxStore() *InMemorySandboxStore {
	return &InMemorySandboxStore{}
}

// Save stores a sandbox request.
func (s *InMemorySandboxStore) Save(req SandboxRequest) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.requests = append(s.requests, req)
	return nil
}

// ListByUser returns the most recent requests for a user, up to limit.
func (s *InMemorySandboxStore) ListByUser(userID string, limit int) ([]SandboxRequest, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var result []SandboxRequest
	// Iterate in reverse to get most recent first.
	for i := len(s.requests) - 1; i >= 0; i-- {
		if s.requests[i].UserID == userID {
			result = append(result, s.requests[i])
			if len(result) >= limit {
				break
			}
		}
	}
	return result, nil
}

// EndpointInfo describes an available API endpoint.
type EndpointInfo struct {
	Method      string `json:"method"`
	Path        string `json:"path"`
	Description string `json:"description"`
}

// SandboxHandler provides an API sandbox for testing.
type SandboxHandler struct {
	proxy *http.Client
	store SandboxStore
}

// NewSandboxHandler creates a new sandbox handler.
func NewSandboxHandler(proxy *http.Client, store SandboxStore) *SandboxHandler {
	return &SandboxHandler{
		proxy: proxy,
		store: store,
	}
}

type executeRequest struct {
	Method  string            `json:"method"`
	URL     string            `json:"url"`
	Headers map[string]string `json:"headers"`
	Body    string            `json:"body"`
	APIKey  string            `json:"api_key"`
}

type executeResponse struct {
	StatusCode int               `json:"status_code"`
	Headers    map[string]string `json:"headers"`
	Body       string            `json:"body"`
	LatencyMs  int64             `json:"latency_ms"`
}

// availableEndpoints is the list of API endpoints available in the sandbox.
var availableEndpoints = []EndpointInfo{
	{Method: "POST", Path: "/api/v1/identity/register", Description: "Register a new identity"},
	{Method: "GET", Path: "/api/v1/identity/verify", Description: "Verify an identity"},
	{Method: "POST", Path: "/api/v1/identity/lookup", Description: "Look up identity by NIK"},
	{Method: "GET", Path: "/api/v1/signing/status", Description: "Check signing status"},
	{Method: "POST", Path: "/api/v1/signing/request", Description: "Request a digital signature"},
}

// ExecuteRequest handles POST /api/v1/portal/sandbox/execute.
func (h *SandboxHandler) ExecuteRequest(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get("X-User-ID")

	var req executeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "Invalid JSON body")
		return
	}

	if req.APIKey == "" {
		writeError(w, http.StatusBadRequest, "invalid_request", "api_key is required")
		return
	}

	if !strings.HasPrefix(req.APIKey, "gp_test_") {
		writeError(w, http.StatusForbidden, "forbidden", "Production keys are not allowed in sandbox; use a gp_test_ key")
		return
	}

	if req.Method == "" || req.URL == "" {
		writeError(w, http.StatusBadRequest, "invalid_request", "method and url are required")
		return
	}

	// Build the proxied request.
	var bodyReader io.Reader
	if req.Body != "" {
		bodyReader = strings.NewReader(req.Body)
	}

	proxyReq, err := http.NewRequestWithContext(r.Context(), req.Method, req.URL, bodyReader)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", fmt.Sprintf("Failed to create request: %v", err))
		return
	}

	for k, v := range req.Headers {
		proxyReq.Header.Set(k, v)
	}
	proxyReq.Header.Set("X-API-Key", req.APIKey)

	start := time.Now()
	resp, err := h.proxy.Do(proxyReq)
	latency := time.Since(start).Milliseconds()

	if err != nil {
		writeError(w, http.StatusBadGateway, "proxy_error", fmt.Sprintf("Request failed: %v", err))
		return
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		writeError(w, http.StatusBadGateway, "proxy_error", "Failed to read response body")
		return
	}

	respHeaders := make(map[string]string)
	for k := range resp.Header {
		respHeaders[k] = resp.Header.Get(k)
	}

	// Record history.
	if userID != "" {
		h.store.Save(SandboxRequest{
			ID:        uuid.New().String(),
			UserID:    userID,
			Method:    req.Method,
			URL:       req.URL,
			Status:    resp.StatusCode,
			LatencyMs: latency,
			CreatedAt: time.Now().UTC(),
		})
	}

	result := executeResponse{
		StatusCode: resp.StatusCode,
		Headers:    respHeaders,
		Body:       string(respBody),
		LatencyMs:  latency,
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(result)
}

// ListEndpoints handles GET /api/v1/portal/sandbox/endpoints.
func (h *SandboxHandler) ListEndpoints(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]any{"endpoints": availableEndpoints})
}

// GetRequestHistory handles GET /api/v1/portal/sandbox/history.
func (h *SandboxHandler) GetRequestHistory(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get("X-User-ID")
	if userID == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized", "X-User-ID header is required")
		return
	}

	requests, err := h.store.ListByUser(userID, 50)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to retrieve history")
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]any{"requests": requests})
}
