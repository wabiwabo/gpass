package httputil

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
)

// BatchRequest represents a single operation within a batch.
type BatchRequest struct {
	ID      string            `json:"id"`
	Method  string            `json:"method"`
	Path    string            `json:"path"`
	Headers map[string]string `json:"headers,omitempty"`
	Body    json.RawMessage   `json:"body,omitempty"`
}

// BatchResponse represents the result of a single batch operation.
type BatchResponse struct {
	ID         string            `json:"id"`
	StatusCode int               `json:"status_code"`
	Headers    map[string]string `json:"headers,omitempty"`
	Body       json.RawMessage   `json:"body"`
}

type batchPayload struct {
	Requests []BatchRequest `json:"requests"`
}

type batchResult struct {
	Responses []BatchResponse `json:"responses"`
}

// BatchHandler processes multiple API requests in a single HTTP call.
// POST /api/v1/batch
// Body: { "requests": [ { id, method, path, headers?, body? }, ... ] }
// Response: { "responses": [ { id, status_code, headers, body }, ... ] }
// Max requests per batch is configurable. Processed concurrently.
type BatchHandler struct {
	mux      *http.ServeMux
	maxBatch int
}

// NewBatchHandler creates a new BatchHandler with the given router and max batch size.
func NewBatchHandler(mux *http.ServeMux, maxBatch int) *BatchHandler {
	return &BatchHandler{
		mux:      mux,
		maxBatch: maxBatch,
	}
}

// ServeHTTP processes the batch request.
func (h *BatchHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		WriteError(w, http.StatusMethodNotAllowed, "method_not_allowed", "only POST is allowed")
		return
	}

	var payload batchPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid_json", fmt.Sprintf("invalid JSON: %v", err))
		return
	}

	if len(payload.Requests) > h.maxBatch {
		WriteError(w, http.StatusBadRequest, "batch_too_large",
			fmt.Sprintf("batch size %d exceeds maximum of %d", len(payload.Requests), h.maxBatch))
		return
	}

	responses := make([]BatchResponse, len(payload.Requests))
	var wg sync.WaitGroup

	for i, req := range payload.Requests {
		wg.Add(1)
		go func(idx int, br BatchRequest) {
			defer wg.Done()
			responses[idx] = h.executeRequest(br)
		}(i, req)
	}

	wg.Wait()

	WriteJSON(w, http.StatusOK, batchResult{Responses: responses})
}

func (h *BatchHandler) executeRequest(br BatchRequest) BatchResponse {
	var body io.Reader
	if len(br.Body) > 0 {
		body = bytes.NewReader(br.Body)
	}

	req, err := http.NewRequest(br.Method, br.Path, body)
	if err != nil {
		errBody, _ := json.Marshal(ErrorResponse{
			Error:   "request_error",
			Message: err.Error(),
		})
		return BatchResponse{
			ID:         br.ID,
			StatusCode: http.StatusBadRequest,
			Body:       errBody,
		}
	}

	for k, v := range br.Headers {
		req.Header.Set(k, v)
	}
	if req.Header.Get("Content-Type") == "" && len(br.Body) > 0 {
		req.Header.Set("Content-Type", "application/json")
	}

	rec := httptest.NewRecorder()
	h.mux.ServeHTTP(rec, req)

	result := rec.Result()
	respHeaders := make(map[string]string, len(result.Header))
	for k := range result.Header {
		respHeaders[k] = result.Header.Get(k)
	}

	respBody := rec.Body.Bytes()
	// Ensure body is valid JSON; if not, wrap it.
	if !json.Valid(respBody) {
		respBody, _ = json.Marshal(string(respBody))
	}

	return BatchResponse{
		ID:         br.ID,
		StatusCode: result.StatusCode,
		Headers:    respHeaders,
		Body:       respBody,
	}
}
