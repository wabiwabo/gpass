package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/garudapass/gpass/services/garudainfo/store"
)

func TestGetConsentScreen_ValidFields(t *testing.T) {
	s := store.NewInMemoryConsentStore()
	h := NewScreenHandler(s)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/consent/screen?app_id=test-app-123&scope=name,dob,address&purpose=kyc_verification", nil)
	w := httptest.NewRecorder()

	h.GetConsentScreen(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	var resp consentScreenResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decoding response: %v", err)
	}

	if resp.App.Name != "Application test-app-123" {
		t.Errorf("unexpected app name: %s", resp.App.Name)
	}

	if len(resp.RequestedFields) != 3 {
		t.Fatalf("expected 3 fields, got %d", len(resp.RequestedFields))
	}

	// Check field order and metadata.
	expected := []struct {
		field    string
		label    string
		required bool
	}{
		{"name", "Nama Lengkap", true},
		{"dob", "Tanggal Lahir", true},
		{"address", "Alamat", false},
	}

	for i, e := range expected {
		f := resp.RequestedFields[i]
		if f.Field != e.field {
			t.Errorf("field[%d]: expected field=%s, got %s", i, e.field, f.Field)
		}
		if f.Label != e.label {
			t.Errorf("field[%d]: expected label=%s, got %s", i, e.label, f.Label)
		}
		if f.Required != e.required {
			t.Errorf("field[%d]: expected required=%v, got %v", i, e.required, f.Required)
		}
	}

	if resp.Purpose != "kyc_verification" {
		t.Errorf("expected purpose=kyc_verification, got %s", resp.Purpose)
	}
	if resp.PurposeLabel != "Verifikasi Identitas (KYC)" {
		t.Errorf("expected purpose_label=Verifikasi Identitas (KYC), got %s", resp.PurposeLabel)
	}
	if resp.ExpiresIn != "365d" {
		t.Errorf("expected expires_in=365d, got %s", resp.ExpiresIn)
	}
}

func TestGetConsentScreen_UnknownFieldsExcluded(t *testing.T) {
	s := store.NewInMemoryConsentStore()
	h := NewScreenHandler(s)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/consent/screen?app_id=app-1&scope=name,unknown_field,dob&purpose=kyc_verification", nil)
	w := httptest.NewRecorder()

	h.GetConsentScreen(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	var resp consentScreenResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decoding response: %v", err)
	}

	if len(resp.RequestedFields) != 2 {
		t.Fatalf("expected 2 fields (unknown excluded), got %d", len(resp.RequestedFields))
	}

	if resp.RequestedFields[0].Field != "name" {
		t.Errorf("expected first field=name, got %s", resp.RequestedFields[0].Field)
	}
	if resp.RequestedFields[1].Field != "dob" {
		t.Errorf("expected second field=dob, got %s", resp.RequestedFields[1].Field)
	}
}

func TestGetConsentScreen_UnknownPurposeUsesRawCode(t *testing.T) {
	s := store.NewInMemoryConsentStore()
	h := NewScreenHandler(s)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/consent/screen?app_id=app-1&scope=name&purpose=custom_purpose", nil)
	w := httptest.NewRecorder()

	h.GetConsentScreen(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	var resp consentScreenResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decoding response: %v", err)
	}

	if resp.PurposeLabel != "custom_purpose" {
		t.Errorf("expected purpose_label=custom_purpose for unknown purpose, got %s", resp.PurposeLabel)
	}
}

func TestGetConsentScreen_MissingAppID(t *testing.T) {
	s := store.NewInMemoryConsentStore()
	h := NewScreenHandler(s)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/consent/screen?scope=name&purpose=kyc_verification", nil)
	w := httptest.NewRecorder()

	h.GetConsentScreen(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

func TestGetConsentScreen_MissingScope(t *testing.T) {
	s := store.NewInMemoryConsentStore()
	h := NewScreenHandler(s)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/consent/screen?app_id=app-1&purpose=kyc_verification", nil)
	w := httptest.NewRecorder()

	h.GetConsentScreen(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}
