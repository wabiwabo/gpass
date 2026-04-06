package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestVerifyNIK_InvalidFormat_TooShort(t *testing.T) {
	rec := postJSON(t, VerifyNIK, VerifyNIKRequest{NIK: "12345"})

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var resp VerifyNIKResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Valid {
		t.Error("expected valid=false for short NIK format")
	}
}

func TestVerifyNIK_InvalidFormat_NonNumeric(t *testing.T) {
	rec := postJSON(t, VerifyNIK, VerifyNIKRequest{NIK: "ABCDEFGHIJKLMNOP"})

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var resp VerifyNIKResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Valid {
		t.Error("expected valid=false for non-numeric NIK")
	}
}

func TestVerifyNIK_NonExistent(t *testing.T) {
	rec := postJSON(t, VerifyNIK, VerifyNIKRequest{NIK: "1111111111111111"})

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var resp VerifyNIKResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Valid {
		t.Error("expected valid=false for non-existent NIK")
	}
	if resp.Alive {
		t.Error("expected alive=false for non-existent NIK")
	}
	if resp.Province != "" {
		t.Errorf("expected empty province, got %s", resp.Province)
	}
}

func TestVerifyNIK_ResponseFormatAllFields(t *testing.T) {
	rec := postJSON(t, VerifyNIK, VerifyNIKRequest{NIK: "3201011501900001"})

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	// Decode into raw map to check all fields are present
	var raw map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&raw); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	requiredFields := []string{"valid", "alive", "province"}
	for _, field := range requiredFields {
		if _, ok := raw[field]; !ok {
			t.Errorf("response missing required field %q", field)
		}
	}
}

func TestVerifyNIK_MultipleSequentialLookups(t *testing.T) {
	niks := []string{
		"3201011501900001",
		"3174015506850002",
		"3507012003950003",
		"5171014712880004",
		"1271010110750005",
		"3301016502600006",
	}

	for _, nik := range niks {
		rec := postJSON(t, VerifyNIK, VerifyNIKRequest{NIK: nik})
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 for NIK %s, got %d", nik, rec.Code)
		}

		var resp VerifyNIKResponse
		if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
			t.Fatalf("failed to decode response for NIK %s: %v", nik, err)
		}

		if !resp.Valid {
			t.Errorf("expected valid=true for NIK %s", nik)
		}
	}
}

func TestVerifyNIK_ContentTypeHeader(t *testing.T) {
	rec := postJSON(t, VerifyNIK, VerifyNIKRequest{NIK: "3201011501900001"})

	ct := rec.Header().Get("Content-Type")
	if ct != "application/json; charset=utf-8" {
		t.Errorf("expected Content-Type 'application/json; charset=utf-8', got %q", ct)
	}

	xct := rec.Header().Get("X-Content-Type-Options")
	if xct != "nosniff" {
		t.Errorf("expected X-Content-Type-Options 'nosniff', got %q", xct)
	}

	cc := rec.Header().Get("Cache-Control")
	if cc != "no-store, no-cache, must-revalidate, private" {
		t.Errorf("expected Cache-Control 'no-store, no-cache, must-revalidate, private', got %q", cc)
	}
}

func TestVerifyNIK_InvalidJSONBody(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("not json at all"))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	VerifyNIK(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid JSON, got %d", rec.Code)
	}

	var resp ErrorResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode error response: %v", err)
	}

	if resp.Error != "bad_request" {
		t.Errorf("expected error 'bad_request', got %q", resp.Error)
	}
}

func TestVerifyDemographic_EmptyNIK(t *testing.T) {
	rec := postJSON(t, VerifyDemographic, VerifyDemographicRequest{
		NIK:    "",
		Name:   "BUDI SANTOSO",
		DOB:    "1990-01-15",
		Gender: "M",
	})

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for empty NIK, got %d", rec.Code)
	}
}
