package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestExportData_Success(t *testing.T) {
	h := NewExportHandler()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/identity/export", nil)
	req.Header.Set("X-User-ID", "user-123")
	rec := httptest.NewRecorder()

	h.ExportData(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
	}

	var resp exportResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.ExportTimestamp.IsZero() {
		t.Error("expected non-zero export_timestamp")
	}

	if len(resp.DataCategories) == 0 {
		t.Error("expected non-empty data_categories")
	}

	if resp.PersonalData.UserID != "user-123" {
		t.Errorf("expected user_id %q, got %q", "user-123", resp.PersonalData.UserID)
	}

	if resp.PersonalData.MaskedNIK == "" {
		t.Error("expected non-empty masked_nik")
	}

	if resp.PersonalData.VerificationStatus == "" {
		t.Error("expected non-empty verification_status")
	}

	if len(resp.PersonalData.ConsentList) == 0 {
		t.Error("expected non-empty consent_list")
	}

	// Verify Cache-Control header
	if cc := rec.Header().Get("Cache-Control"); cc != "no-store" {
		t.Errorf("expected Cache-Control %q, got %q", "no-store", cc)
	}
}

func TestExportData_MissingUserID(t *testing.T) {
	h := NewExportHandler()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/identity/export", nil)
	rec := httptest.NewRecorder()

	h.ExportData(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}

	var resp map[string]string
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp["error"] != "missing_user_id" {
		t.Errorf("expected error %q, got %q", "missing_user_id", resp["error"])
	}
}
