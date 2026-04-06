package apiresponse

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/garudapass/gpass/packages/golib/errors"
)

func TestNewProblem(t *testing.T) {
	p := NewProblem(http.StatusBadRequest, "Bad Request", "invalid input")

	if p.Type != "about:blank" {
		t.Errorf("Type = %q, want %q", p.Type, "about:blank")
	}
	if p.Title != "Bad Request" {
		t.Errorf("Title = %q, want %q", p.Title, "Bad Request")
	}
	if p.Status != http.StatusBadRequest {
		t.Errorf("Status = %d, want %d", p.Status, http.StatusBadRequest)
	}
	if p.Detail != "invalid input" {
		t.Errorf("Detail = %q, want %q", p.Detail, "invalid input")
	}
	if p.Instance != "" {
		t.Errorf("Instance = %q, want empty", p.Instance)
	}
}

func TestWriteProblem(t *testing.T) {
	p := NewProblem(http.StatusNotFound, "Not Found", "resource missing")
	w := httptest.NewRecorder()

	WriteProblem(w, p)

	resp := w.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusNotFound)
	}
	ct := resp.Header.Get("Content-Type")
	if ct != "application/problem+json" {
		t.Errorf("Content-Type = %q, want %q", ct, "application/problem+json")
	}

	var got Problem
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.Status != http.StatusNotFound {
		t.Errorf("body status = %d, want %d", got.Status, http.StatusNotFound)
	}
	if got.Title != "Not Found" {
		t.Errorf("body title = %q, want %q", got.Title, "Not Found")
	}
}

func TestWriteSuccess(t *testing.T) {
	data := map[string]string{"name": "test"}
	w := httptest.NewRecorder()

	WriteSuccess(w, data, nil)

	resp := w.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	ct := resp.Header.Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("Content-Type = %q, want %q", ct, "application/json")
	}

	var got map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	d, ok := got["data"].(map[string]interface{})
	if !ok {
		t.Fatal("data field missing or wrong type")
	}
	if d["name"] != "test" {
		t.Errorf("data.name = %v, want %q", d["name"], "test")
	}
}

func TestWriteCreated(t *testing.T) {
	data := map[string]string{"id": "abc"}
	w := httptest.NewRecorder()

	WriteCreated(w, data)

	resp := w.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusCreated)
	}

	var got map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	d, ok := got["data"].(map[string]interface{})
	if !ok {
		t.Fatal("data field missing or wrong type")
	}
	if d["id"] != "abc" {
		t.Errorf("data.id = %v, want %q", d["id"], "abc")
	}
}

func TestWriteNoContent(t *testing.T) {
	w := httptest.NewRecorder()

	WriteNoContent(w)

	resp := w.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusNoContent)
	}
	body, _ := io.ReadAll(resp.Body)
	if len(body) != 0 {
		t.Errorf("body = %q, want empty", string(body))
	}
}

func TestWriteList_WithMeta(t *testing.T) {
	items := []string{"a", "b", "c"}
	meta := Meta{
		Page:       1,
		PerPage:    10,
		TotalCount: 3,
		TotalPages: 1,
	}
	w := httptest.NewRecorder()

	WriteList(w, items, meta)

	resp := w.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var got map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}

	// Check data is present.
	dataArr, ok := got["data"].([]interface{})
	if !ok {
		t.Fatal("data field missing or not an array")
	}
	if len(dataArr) != 3 {
		t.Errorf("data length = %d, want 3", len(dataArr))
	}

	// Check meta is present.
	m, ok := got["meta"].(map[string]interface{})
	if !ok {
		t.Fatal("meta field missing or wrong type")
	}
	if int(m["total_count"].(float64)) != 3 {
		t.Errorf("meta.total_count = %v, want 3", m["total_count"])
	}
	if int(m["page"].(float64)) != 1 {
		t.Errorf("meta.page = %v, want 1", m["page"])
	}
}

func TestValidationProblem(t *testing.T) {
	fields := map[string]string{
		"email": "invalid format",
		"name":  "required",
	}
	p := ValidationProblem("validation failed", fields)

	if p.Status != http.StatusUnprocessableEntity {
		t.Errorf("Status = %d, want %d", p.Status, http.StatusUnprocessableEntity)
	}
	if p.Title != "Validation Error" {
		t.Errorf("Title = %q, want %q", p.Title, "Validation Error")
	}

	// Verify field errors are included when serialized.
	data, err := json.Marshal(p)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	f, ok := raw["fields"].(map[string]interface{})
	if !ok {
		t.Fatal("fields extension missing")
	}
	if f["email"] != "invalid format" {
		t.Errorf("fields.email = %v, want %q", f["email"], "invalid format")
	}
	if f["name"] != "required" {
		t.Errorf("fields.name = %v, want %q", f["name"], "required")
	}
}

func TestNotFoundProblem(t *testing.T) {
	p := NotFoundProblem("User")

	if p.Status != http.StatusNotFound {
		t.Errorf("Status = %d, want %d", p.Status, http.StatusNotFound)
	}
	if p.Detail != "User not found" {
		t.Errorf("Detail = %q, want %q", p.Detail, "User not found")
	}
}

func TestRateLimitProblem(t *testing.T) {
	w := httptest.NewRecorder()
	WriteRateLimitProblem(w, 60)

	resp := w.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusTooManyRequests {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusTooManyRequests)
	}
	ra := resp.Header.Get("Retry-After")
	if ra != "60" {
		t.Errorf("Retry-After = %q, want %q", ra, "60")
	}

	var got map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if int(got["retry_after"].(float64)) != 60 {
		t.Errorf("retry_after = %v, want 60", got["retry_after"])
	}
}

func TestFromAppError(t *testing.T) {
	appErr := errors.BadRequest("invalid_request", "missing field")
	p := FromAppError(appErr)

	if p.Status != http.StatusBadRequest {
		t.Errorf("Status = %d, want %d", p.Status, http.StatusBadRequest)
	}
	if p.Title != "invalid_request" {
		t.Errorf("Title = %q, want %q", p.Title, "invalid_request")
	}
	if p.Detail != "missing field" {
		t.Errorf("Detail = %q, want %q", p.Detail, "missing field")
	}
}

func TestProblem_Extensions(t *testing.T) {
	p := NewProblem(http.StatusConflict, "Conflict", "duplicate entry")
	p.Extensions = map[string]interface{}{
		"trace_id":    "abc-123",
		"retry_count": 3,
	}

	data, err := json.Marshal(p)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if raw["trace_id"] != "abc-123" {
		t.Errorf("trace_id = %v, want %q", raw["trace_id"], "abc-123")
	}
	if int(raw["retry_count"].(float64)) != 3 {
		t.Errorf("retry_count = %v, want 3", raw["retry_count"])
	}
	// Standard fields should still be present.
	if raw["title"] != "Conflict" {
		t.Errorf("title = %v, want %q", raw["title"], "Conflict")
	}
}
