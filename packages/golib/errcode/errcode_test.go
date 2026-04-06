package errcode

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRegistry_RegisterGet(t *testing.T) {
	r := NewRegistry()
	r.Register("not_found", 404, "Resource not found")

	code, ok := r.Get("not_found")
	if !ok {
		t.Fatal("should find code")
	}
	if code.Status != 404 {
		t.Errorf("status: got %d", code.Status)
	}
	if code.Message != "Resource not found" {
		t.Errorf("message: got %q", code.Message)
	}
}

func TestRegistry_GetMissing(t *testing.T) {
	r := NewRegistry()
	_, ok := r.Get("nonexistent")
	if ok {
		t.Error("should not find missing code")
	}
}

func TestRegistry_Write(t *testing.T) {
	r := NewRegistry()
	r.Register("validation_failed", 400, "Validation failed")

	w := httptest.NewRecorder()
	r.Write(w, "validation_failed")

	if w.Code != 400 {
		t.Errorf("status: got %d", w.Code)
	}

	var body map[string]interface{}
	json.NewDecoder(w.Body).Decode(&body)
	if body["code"] != "validation_failed" {
		t.Errorf("code: got %v", body["code"])
	}
	if body["status"] != float64(400) {
		t.Errorf("body status: got %v", body["status"])
	}
}

func TestRegistry_WriteUnknown(t *testing.T) {
	r := NewRegistry()
	w := httptest.NewRecorder()
	r.Write(w, "unknown_code")

	if w.Code != 500 {
		t.Errorf("unknown code: got %d", w.Code)
	}
}

func TestRegistry_WriteWithDetail(t *testing.T) {
	r := NewRegistry()
	r.Register("not_found", 404, "Not found")

	w := httptest.NewRecorder()
	r.WriteWithDetail(w, "not_found", "User with ID 123 not found")

	var body map[string]interface{}
	json.NewDecoder(w.Body).Decode(&body)
	if body["detail"] != "User with ID 123 not found" {
		t.Errorf("detail: got %v", body["detail"])
	}
}

func TestRegistry_All(t *testing.T) {
	r := NewRegistry()
	r.Register("a", 400, "A")
	r.Register("b", 404, "B")

	all := r.All()
	if len(all) != 2 {
		t.Errorf("all: got %d", len(all))
	}
}

func TestRegistry_Count(t *testing.T) {
	r := NewRegistry()
	r.Register("a", 400, "A")
	if r.Count() != 1 {
		t.Errorf("count: got %d", r.Count())
	}
}

func TestDefault(t *testing.T) {
	r := Default()
	if r.Count() < 20 {
		t.Errorf("default should have 20+ codes: got %d", r.Count())
	}

	// Check Indonesian-specific codes.
	code, ok := r.Get("nik_invalid")
	if !ok {
		t.Error("should have nik_invalid")
	}
	if code.Status != 400 {
		t.Errorf("nik_invalid status: got %d", code.Status)
	}

	code, ok = r.Get("consent_required")
	if !ok {
		t.Error("should have consent_required")
	}
	if code.Status != 403 {
		t.Errorf("consent_required status: got %d", code.Status)
	}
}

func TestCode_Error(t *testing.T) {
	c := Code{Code: "not_found", Status: 404, Message: "Not found"}
	if c.Error() != "[not_found] Not found" {
		t.Errorf("Error(): got %q", c.Error())
	}
}

func TestRegistry_Headers(t *testing.T) {
	r := NewRegistry()
	r.Register("test", 400, "Test")

	w := httptest.NewRecorder()
	r.Write(w, "test")

	if ct := w.Header().Get("Content-Type"); ct != "application/problem+json" {
		t.Errorf("Content-Type: got %q", ct)
	}
	if cc := w.Header().Get("Cache-Control"); cc != "no-store" {
		t.Errorf("Cache-Control: got %q", cc)
	}
}

func TestDefault_AuthCodes(t *testing.T) {
	r := Default()
	for _, code := range []string{"auth_required", "auth_invalid", "auth_expired", "token_invalid"} {
		if _, ok := r.Get(code); !ok {
			t.Errorf("missing auth code: %s", code)
		}
	}
}

func TestDefault_RateLimitCodes(t *testing.T) {
	r := Default()
	code, _ := r.Get("rate_limited")
	if code.Status != http.StatusTooManyRequests {
		t.Errorf("rate_limited status: got %d", code.Status)
	}
}
