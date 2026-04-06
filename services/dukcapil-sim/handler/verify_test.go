package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func postJSON(t *testing.T, handler http.HandlerFunc, body interface{}) *httptest.ResponseRecorder {
	t.Helper()
	b, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("failed to marshal request body: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler(rec, req)
	return rec
}

func TestVerifyNIK_Valid(t *testing.T) {
	rec := postJSON(t, VerifyNIK, VerifyNIKRequest{NIK: "3201011501900001"})

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var resp VerifyNIKResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if !resp.Valid {
		t.Error("expected valid=true")
	}
	if !resp.Alive {
		t.Error("expected alive=true")
	}
	if resp.Province != "JAWA BARAT" {
		t.Errorf("expected province JAWA BARAT, got %s", resp.Province)
	}
}

func TestVerifyNIK_NotFound(t *testing.T) {
	rec := postJSON(t, VerifyNIK, VerifyNIKRequest{NIK: "9999999999999999"})

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var resp VerifyNIKResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Valid {
		t.Error("expected valid=false for unknown NIK")
	}
}

func TestVerifyNIK_Deceased(t *testing.T) {
	rec := postJSON(t, VerifyNIK, VerifyNIKRequest{NIK: "3301016502600006"})

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var resp VerifyNIKResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if !resp.Valid {
		t.Error("expected valid=true for deceased person (NIK still valid)")
	}
	if resp.Alive {
		t.Error("expected alive=false for deceased person")
	}
	if resp.Province != "JAWA TENGAH" {
		t.Errorf("expected province JAWA TENGAH, got %s", resp.Province)
	}
}

func TestVerifyDemographic_FullMatch(t *testing.T) {
	rec := postJSON(t, VerifyDemographic, VerifyDemographicRequest{
		NIK:    "3201011501900001",
		Name:   "BUDI SANTOSO",
		DOB:    "1990-01-15",
		Gender: "M",
	})

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var resp VerifyDemographicResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if !resp.Match {
		t.Error("expected match=true for exact demographic match")
	}
	if resp.Confidence != 1.0 {
		t.Errorf("expected confidence=1.0, got %f", resp.Confidence)
	}
}

func TestVerifyDemographic_PartialMatch(t *testing.T) {
	rec := postJSON(t, VerifyDemographic, VerifyDemographicRequest{
		NIK:    "3201011501900001",
		Name:   "BUDI SANTOSO",
		DOB:    "1990-01-15",
		Gender: "F", // wrong gender
	})

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var resp VerifyDemographicResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Match {
		t.Error("expected match=false for partial match")
	}
	// 2 out of 3 fields match
	expected := 2.0 / 3.0
	if resp.Confidence < expected-0.01 || resp.Confidence > expected+0.01 {
		t.Errorf("expected confidence ~%f, got %f", expected, resp.Confidence)
	}
}

func TestVerifyBiometric_Match(t *testing.T) {
	rec := postJSON(t, VerifyBiometric, VerifyBiometricRequest{
		NIK:          "3201011501900001",
		SelfieBase64: "aVZCT1J3MEtHZ29BQUFBTlNVaEVVZ0FBQU1BQUFBREQ=", // matches stored photo
	})

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var resp VerifyBiometricResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if !resp.Match {
		t.Error("expected match=true for exact photo match")
	}
	if resp.Score != 0.92 {
		t.Errorf("expected score=0.92, got %f", resp.Score)
	}
}

func TestVerifyBiometric_Mismatch(t *testing.T) {
	rec := postJSON(t, VerifyBiometric, VerifyBiometricRequest{
		NIK:          "3201011501900001",
		SelfieBase64: "dGhpc2lzYWRpZmZlcmVudHBob3Rv", // different photo
	})

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var resp VerifyBiometricResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Match {
		t.Error("expected match=false for photo mismatch")
	}
	if resp.Score != 0.21 {
		t.Errorf("expected score=0.21, got %f", resp.Score)
	}
}

func TestVerifyNIK_EmptyNIK(t *testing.T) {
	rec := postJSON(t, VerifyNIK, VerifyNIKRequest{NIK: ""})

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestVerifyBiometric_NotFound(t *testing.T) {
	rec := postJSON(t, VerifyBiometric, VerifyBiometricRequest{
		NIK:          "9999999999999999",
		SelfieBase64: "c29tZXBob3Rv",
	})

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var resp VerifyBiometricResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Match {
		t.Error("expected match=false for unknown NIK")
	}
	if resp.Score != 0 {
		t.Errorf("expected score=0, got %f", resp.Score)
	}
}
