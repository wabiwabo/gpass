package structured

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNewError(t *testing.T) {
	err := NewError(400, "Bad Request").
		Detail("Invalid input").
		Code("INVALID_INPUT").
		Build()

	if err.Status != 400 {
		t.Errorf("status: got %d", err.Status)
	}
	if err.Title != "Bad Request" {
		t.Errorf("title: got %q", err.Title)
	}
	if err.Detail != "Invalid input" {
		t.Errorf("detail: got %q", err.Detail)
	}
	if err.Code != "INVALID_INPUT" {
		t.Errorf("code: got %q", err.Code)
	}
}

func TestBadRequest(t *testing.T) {
	err := BadRequest("Validation Failed").Build()
	if err.Status != 400 {
		t.Errorf("status: got %d", err.Status)
	}
}

func TestNotFound(t *testing.T) {
	err := NotFound("Resource Not Found").Build()
	if err.Status != 404 {
		t.Errorf("status: got %d", err.Status)
	}
}

func TestForbidden(t *testing.T) {
	err := Forbidden("Access Denied").Build()
	if err.Status != 403 {
		t.Errorf("status: got %d", err.Status)
	}
}

func TestUnauthorized(t *testing.T) {
	err := Unauthorized("Authentication Required").Build()
	if err.Status != 401 {
		t.Errorf("status: got %d", err.Status)
	}
}

func TestConflict(t *testing.T) {
	err := Conflict("Resource Conflict").Build()
	if err.Status != 409 {
		t.Errorf("status: got %d", err.Status)
	}
}

func TestInternalError(t *testing.T) {
	err := InternalError("Server Error").Build()
	if err.Status != 500 {
		t.Errorf("status: got %d", err.Status)
	}
}

func TestBuilder_Field(t *testing.T) {
	err := BadRequest("Validation Failed").
		Field("email", "required", "Email is required").
		Field("name", "min_length", "Name must be at least 2 characters").
		Build()

	if !err.HasFields() {
		t.Error("should have fields")
	}
	if err.FieldCount() != 2 {
		t.Errorf("field count: got %d", err.FieldCount())
	}
	if err.Fields[0].Field != "email" {
		t.Errorf("field[0]: got %q", err.Fields[0].Field)
	}
}

func TestBuilder_Meta(t *testing.T) {
	err := BadRequest("Error").
		Meta("request_id", "req-123").
		Meta("trace_id", "trace-456").
		Build()

	if err.Meta["request_id"] != "req-123" {
		t.Errorf("meta: got %v", err.Meta)
	}
}

func TestBuilder_Type(t *testing.T) {
	err := BadRequest("Error").
		Type("https://api.garudapass.id/errors/validation").
		Build()

	if err.Type != "https://api.garudapass.id/errors/validation" {
		t.Errorf("type: got %q", err.Type)
	}
}

func TestBuilder_Instance(t *testing.T) {
	err := NotFound("Not Found").
		Instance("/api/users/123").
		Build()

	if err.Instance != "/api/users/123" {
		t.Errorf("instance: got %q", err.Instance)
	}
}

func TestError_ErrorInterface(t *testing.T) {
	err := BadRequest("Bad Request").Detail("Missing field").Build()

	if err.Error() != "Bad Request: Missing field" {
		t.Errorf("Error(): got %q", err.Error())
	}

	err2 := NotFound("Not Found").Build()
	if err2.Error() != "Not Found" {
		t.Errorf("Error() without detail: got %q", err2.Error())
	}
}

func TestWriteError(t *testing.T) {
	err := BadRequest("Validation Failed").
		Field("email", "required", "Email is required").
		Build()

	w := httptest.NewRecorder()
	WriteError(w, err)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status: got %d", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/problem+json" {
		t.Errorf("Content-Type: got %q", ct)
	}
	if cc := w.Header().Get("Cache-Control"); cc != "no-store" {
		t.Errorf("Cache-Control: got %q", cc)
	}

	var body Error
	json.NewDecoder(w.Body).Decode(&body)
	if body.Status != 400 {
		t.Errorf("body status: got %d", body.Status)
	}
	if len(body.Fields) != 1 {
		t.Errorf("body fields: got %d", len(body.Fields))
	}
}

func TestBuilder_Write(t *testing.T) {
	w := httptest.NewRecorder()
	InternalError("Server Error").Detail("Something broke").Write(w)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status: got %d", w.Code)
	}
}

func TestError_NoFields(t *testing.T) {
	err := BadRequest("Error").Build()
	if err.HasFields() {
		t.Error("should not have fields")
	}
	if err.FieldCount() != 0 {
		t.Error("field count should be 0")
	}
}

func TestError_DefaultType(t *testing.T) {
	err := BadRequest("Error").Build()
	if err.Type != "about:blank" {
		t.Errorf("default type: got %q", err.Type)
	}
}
