package respenvelope

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestOK(t *testing.T) {
	w := httptest.NewRecorder()
	OK(w, map[string]string{"name": "Budi"})

	if w.Code != http.StatusOK {
		t.Errorf("status: got %d", w.Code)
	}

	var env Envelope
	json.NewDecoder(w.Body).Decode(&env)
	if !env.Success {
		t.Error("should be success")
	}
	if env.Error != nil {
		t.Error("should not have error")
	}
	if env.Timestamp.IsZero() {
		t.Error("should have timestamp")
	}
}

func TestOKWithMeta(t *testing.T) {
	w := httptest.NewRecorder()
	OKWithMeta(w, []string{"a", "b"}, Meta{Page: 1, PerPage: 10, Total: 100, TotalPages: 10})

	var env Envelope
	json.NewDecoder(w.Body).Decode(&env)
	if env.Meta == nil {
		t.Error("should have meta")
	}
}

func TestCreated(t *testing.T) {
	w := httptest.NewRecorder()
	Created(w, map[string]string{"id": "123"})

	if w.Code != http.StatusCreated {
		t.Errorf("status: got %d", w.Code)
	}
}

func TestNoContent(t *testing.T) {
	w := httptest.NewRecorder()
	NoContent(w)

	if w.Code != http.StatusNoContent {
		t.Errorf("status: got %d", w.Code)
	}
}

func TestErr(t *testing.T) {
	w := httptest.NewRecorder()
	Err(w, http.StatusBadRequest, "invalid_input", "Name is required")

	if w.Code != http.StatusBadRequest {
		t.Errorf("status: got %d", w.Code)
	}

	var env Envelope
	json.NewDecoder(w.Body).Decode(&env)
	if env.Success {
		t.Error("should not be success")
	}
	if env.Error == nil {
		t.Fatal("should have error")
	}
	if env.Error.Code != "invalid_input" {
		t.Errorf("code: got %q", env.Error.Code)
	}
}

func TestBadRequest(t *testing.T) {
	w := httptest.NewRecorder()
	BadRequest(w, "missing", "Field required")
	if w.Code != 400 {
		t.Errorf("status: got %d", w.Code)
	}
}

func TestNotFound(t *testing.T) {
	w := httptest.NewRecorder()
	NotFound(w, "User not found")
	if w.Code != 404 {
		t.Errorf("status: got %d", w.Code)
	}
}

func TestForbidden(t *testing.T) {
	w := httptest.NewRecorder()
	Forbidden(w, "Access denied")
	if w.Code != 403 {
		t.Errorf("status: got %d", w.Code)
	}
}

func TestInternalError(t *testing.T) {
	w := httptest.NewRecorder()
	InternalError(w, "Something broke")
	if w.Code != 500 {
		t.Errorf("status: got %d", w.Code)
	}
}

func TestHeaders(t *testing.T) {
	w := httptest.NewRecorder()
	OK(w, nil)

	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type: got %q", ct)
	}
	if cc := w.Header().Get("Cache-Control"); cc != "no-store" {
		t.Errorf("Cache-Control: got %q", cc)
	}
}
