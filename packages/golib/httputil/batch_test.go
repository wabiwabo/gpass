package httputil

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func setupBatchMux() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/hello", func(w http.ResponseWriter, r *http.Request) {
		WriteJSON(w, http.StatusOK, map[string]string{"message": "hello"})
	})
	mux.HandleFunc("/api/echo", func(w http.ResponseWriter, r *http.Request) {
		var body map[string]interface{}
		json.NewDecoder(r.Body).Decode(&body)
		WriteJSON(w, http.StatusOK, body)
	})
	mux.HandleFunc("/api/fail", func(w http.ResponseWriter, r *http.Request) {
		WriteError(w, http.StatusInternalServerError, "error", "something went wrong")
	})
	mux.HandleFunc("/api/slow", func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(50 * time.Millisecond)
		WriteJSON(w, http.StatusOK, map[string]string{"status": "done"})
	})
	return mux
}

func doBatchRequest(t *testing.T, handler http.Handler, payload interface{}) *httptest.ResponseRecorder {
	t.Helper()
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/batch", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}

func TestBatchHandler_SingleRequest(t *testing.T) {
	h := NewBatchHandler(setupBatchMux(), 20)
	payload := batchPayload{
		Requests: []BatchRequest{
			{ID: "1", Method: "GET", Path: "/api/hello"},
		},
	}

	rec := doBatchRequest(t, h, payload)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var result batchResult
	json.NewDecoder(rec.Body).Decode(&result)

	if len(result.Responses) != 1 {
		t.Fatalf("expected 1 response, got %d", len(result.Responses))
	}
	if result.Responses[0].ID != "1" {
		t.Errorf("expected ID '1', got '%s'", result.Responses[0].ID)
	}
	if result.Responses[0].StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", result.Responses[0].StatusCode)
	}
}

func TestBatchHandler_MultipleConcurrent(t *testing.T) {
	mux := http.NewServeMux()
	var counter atomic.Int32
	mux.HandleFunc("/api/count", func(w http.ResponseWriter, r *http.Request) {
		counter.Add(1)
		// Small sleep to allow concurrent execution to overlap
		time.Sleep(20 * time.Millisecond)
		WriteJSON(w, http.StatusOK, map[string]int32{"count": counter.Load()})
	})

	h := NewBatchHandler(mux, 20)
	payload := batchPayload{
		Requests: []BatchRequest{
			{ID: "a", Method: "GET", Path: "/api/count"},
			{ID: "b", Method: "GET", Path: "/api/count"},
			{ID: "c", Method: "GET", Path: "/api/count"},
		},
	}

	start := time.Now()
	rec := doBatchRequest(t, h, payload)
	elapsed := time.Since(start)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var result batchResult
	json.NewDecoder(rec.Body).Decode(&result)

	if len(result.Responses) != 3 {
		t.Fatalf("expected 3 responses, got %d", len(result.Responses))
	}

	// If truly concurrent, 3 x 20ms should complete well under 3 x 20ms = 60ms
	// Allow some margin but should be faster than sequential
	if elapsed > 200*time.Millisecond {
		t.Errorf("batch took %v, expected concurrent execution to be faster", elapsed)
	}
}

func TestBatchHandler_MatchingIDs(t *testing.T) {
	h := NewBatchHandler(setupBatchMux(), 20)
	payload := batchPayload{
		Requests: []BatchRequest{
			{ID: "req-alpha", Method: "GET", Path: "/api/hello"},
			{ID: "req-beta", Method: "GET", Path: "/api/hello"},
		},
	}

	rec := doBatchRequest(t, h, payload)

	var result batchResult
	json.NewDecoder(rec.Body).Decode(&result)

	ids := map[string]bool{}
	for _, r := range result.Responses {
		ids[r.ID] = true
	}
	if !ids["req-alpha"] || !ids["req-beta"] {
		t.Errorf("expected IDs 'req-alpha' and 'req-beta', got %v", ids)
	}
}

func TestBatchHandler_ExceedsMaxBatchSize(t *testing.T) {
	h := NewBatchHandler(setupBatchMux(), 2)
	payload := batchPayload{
		Requests: []BatchRequest{
			{ID: "1", Method: "GET", Path: "/api/hello"},
			{ID: "2", Method: "GET", Path: "/api/hello"},
			{ID: "3", Method: "GET", Path: "/api/hello"},
		},
	}

	rec := doBatchRequest(t, h, payload)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}

	var errResp ErrorResponse
	json.NewDecoder(rec.Body).Decode(&errResp)
	if errResp.Error != "batch_too_large" {
		t.Errorf("expected error 'batch_too_large', got '%s'", errResp.Error)
	}
}

func TestBatchHandler_InvalidJSON(t *testing.T) {
	h := NewBatchHandler(setupBatchMux(), 20)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/batch", bytes.NewBufferString("{invalid"))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}

	var errResp ErrorResponse
	json.NewDecoder(rec.Body).Decode(&errResp)
	if errResp.Error != "invalid_json" {
		t.Errorf("expected error 'invalid_json', got '%s'", errResp.Error)
	}
}

func TestBatchHandler_IndividualFailureIsolated(t *testing.T) {
	h := NewBatchHandler(setupBatchMux(), 20)
	payload := batchPayload{
		Requests: []BatchRequest{
			{ID: "ok", Method: "GET", Path: "/api/hello"},
			{ID: "fail", Method: "GET", Path: "/api/fail"},
			{ID: "ok2", Method: "GET", Path: "/api/hello"},
		},
	}

	rec := doBatchRequest(t, h, payload)

	if rec.Code != http.StatusOK {
		t.Fatalf("batch response should be 200 even with individual failures, got %d", rec.Code)
	}

	var result batchResult
	json.NewDecoder(rec.Body).Decode(&result)

	statusByID := map[string]int{}
	for _, r := range result.Responses {
		statusByID[r.ID] = r.StatusCode
	}

	if statusByID["ok"] != http.StatusOK {
		t.Errorf("expected 'ok' to be 200, got %d", statusByID["ok"])
	}
	if statusByID["fail"] != http.StatusInternalServerError {
		t.Errorf("expected 'fail' to be 500, got %d", statusByID["fail"])
	}
	if statusByID["ok2"] != http.StatusOK {
		t.Errorf("expected 'ok2' to be 200, got %d", statusByID["ok2"])
	}
}

func TestBatchHandler_EmptyRequests(t *testing.T) {
	h := NewBatchHandler(setupBatchMux(), 20)
	payload := batchPayload{
		Requests: []BatchRequest{},
	}

	rec := doBatchRequest(t, h, payload)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var result batchResult
	json.NewDecoder(rec.Body).Decode(&result)

	if len(result.Responses) != 0 {
		t.Errorf("expected 0 responses, got %d", len(result.Responses))
	}
}
