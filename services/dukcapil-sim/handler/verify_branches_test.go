package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestVerifyDemographic_BadJSON pins the JSON-decode error branch.
func TestVerifyDemographic_BadJSON(t *testing.T) {
	req := httptest.NewRequest("POST", "/api/v1/verify/demographic", bytes.NewBufferString("{not json"))
	rec := httptest.NewRecorder()
	VerifyDemographic(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("code = %d", rec.Code)
	}
}

// TestVerifyDemographic_NotFoundReturns200WithMatchFalse pins the
// "person == nil → 200 with match=false" branch — important contract:
// non-existent NIK is NOT a 404, it's a successful query that returns
// no match (so callers can distinguish "no record" from "API error").
func TestVerifyDemographic_NotFoundReturns200(t *testing.T) {
	body := `{"nik":"9999999999999999","name":"X","dob":"2000-01-01","gender":"L"}`
	req := httptest.NewRequest("POST", "/", bytes.NewBufferString(body))
	rec := httptest.NewRecorder()
	VerifyDemographic(rec, req)
	if rec.Code != 200 {
		t.Errorf("code = %d, want 200", rec.Code)
	}
	var resp VerifyDemographicResponse
	json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp.Match || resp.Confidence != 0 {
		t.Errorf("nonexistent NIK should return match=false confidence=0, got %+v", resp)
	}
}

// TestVerifyBiometric_BadJSON pins the JSON-decode error branch.
func TestVerifyBiometric_BadJSON(t *testing.T) {
	req := httptest.NewRequest("POST", "/", bytes.NewBufferString("{nope"))
	rec := httptest.NewRecorder()
	VerifyBiometric(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("code = %d", rec.Code)
	}
}

// TestVerifyBiometric_MissingFields pins the empty NIK + empty selfie
// rejection.
func TestVerifyBiometric_MissingFields(t *testing.T) {
	cases := []string{
		`{}`,
		`{"nik":"x"}`,
		`{"selfie_base64":"y"}`,
	}
	for _, body := range cases {
		req := httptest.NewRequest("POST", "/", bytes.NewBufferString(body))
		rec := httptest.NewRecorder()
		VerifyBiometric(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("body=%s code=%d, want 400", body, rec.Code)
		}
	}
}

// TestWriteJSON_SecurityHeaders pins the security-header contract on
// every JSON response: nosniff + no-store cache control.
func TestWriteJSON_SecurityHeaders(t *testing.T) {
	req := httptest.NewRequest("POST", "/", bytes.NewBufferString(`{"nik":"x"}`))
	rec := httptest.NewRecorder()
	VerifyDemographic(rec, req)
	if rec.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Errorf("X-Content-Type-Options = %q", rec.Header().Get("X-Content-Type-Options"))
	}
	cc := rec.Header().Get("Cache-Control")
	if cc == "" || cc[:8] != "no-store" {
		t.Errorf("Cache-Control = %q", cc)
	}
}
