package respjson

import (
	"encoding/json"
	"net/http/httptest"
	"testing"
)

func TestOK(t *testing.T) {
	w := httptest.NewRecorder()
	OK(w, map[string]string{"msg": "hello"})

	if w.Code != 200 {
		t.Errorf("status = %d", w.Code)
	}
	if w.Header().Get("Content-Type") != "application/json; charset=utf-8" {
		t.Errorf("Content-Type = %q", w.Header().Get("Content-Type"))
	}

	var body map[string]string
	json.NewDecoder(w.Body).Decode(&body)
	if body["msg"] != "hello" {
		t.Errorf("body = %v", body)
	}
}

func TestCreated(t *testing.T) {
	w := httptest.NewRecorder()
	Created(w, map[string]string{"id": "123"})
	if w.Code != 201 {
		t.Errorf("status = %d", w.Code)
	}
}

func TestNoContent(t *testing.T) {
	w := httptest.NewRecorder()
	NoContent(w)
	if w.Code != 204 {
		t.Errorf("status = %d", w.Code)
	}
}

func TestWrite(t *testing.T) {
	w := httptest.NewRecorder()
	Write(w, 202, map[string]string{"status": "accepted"})
	if w.Code != 202 {
		t.Errorf("status = %d", w.Code)
	}
}

func TestError(t *testing.T) {
	w := httptest.NewRecorder()
	Error(w, 400, "validation_failed", "email is required")

	if w.Code != 400 {
		t.Errorf("status = %d", w.Code)
	}

	var body map[string]interface{}
	json.NewDecoder(w.Body).Decode(&body)
	if body["error"] != "validation_failed" {
		t.Errorf("error = %v", body["error"])
	}
	if body["message"] != "email is required" {
		t.Errorf("message = %v", body["message"])
	}
}

func TestErrorWithDetail(t *testing.T) {
	w := httptest.NewRecorder()
	ErrorWithDetail(w, 422, "invalid_nik", "NIK invalid", "province 99 not valid")

	var body map[string]interface{}
	json.NewDecoder(w.Body).Decode(&body)
	if body["detail"] != "province 99 not valid" {
		t.Errorf("detail = %v", body["detail"])
	}
}

func TestBadRequest(t *testing.T) {
	w := httptest.NewRecorder()
	BadRequest(w, "bad", "bad request")
	if w.Code != 400 {
		t.Errorf("status = %d", w.Code)
	}
}

func TestUnauthorized(t *testing.T) {
	w := httptest.NewRecorder()
	Unauthorized(w, "auth required")
	if w.Code != 401 {
		t.Errorf("status = %d", w.Code)
	}
}

func TestForbidden(t *testing.T) {
	w := httptest.NewRecorder()
	Forbidden(w, "access denied")
	if w.Code != 403 {
		t.Errorf("status = %d", w.Code)
	}
}

func TestNotFound(t *testing.T) {
	w := httptest.NewRecorder()
	NotFound(w, "user not found")
	if w.Code != 404 {
		t.Errorf("status = %d", w.Code)
	}
}

func TestConflict(t *testing.T) {
	w := httptest.NewRecorder()
	Conflict(w, "already exists")
	if w.Code != 409 {
		t.Errorf("status = %d", w.Code)
	}
}

func TestTooManyRequests(t *testing.T) {
	w := httptest.NewRecorder()
	TooManyRequests(w, "rate limited")
	if w.Code != 429 {
		t.Errorf("status = %d", w.Code)
	}
}

func TestInternal(t *testing.T) {
	w := httptest.NewRecorder()
	Internal(w, "server error")
	if w.Code != 500 {
		t.Errorf("status = %d", w.Code)
	}
}

func TestServiceUnavailable(t *testing.T) {
	w := httptest.NewRecorder()
	ServiceUnavailable(w, "down for maintenance")
	if w.Code != 503 {
		t.Errorf("status = %d", w.Code)
	}
}

func TestOK_Nil(t *testing.T) {
	w := httptest.NewRecorder()
	OK(w, nil)
	if w.Code != 200 {
		t.Errorf("status = %d", w.Code)
	}
}
