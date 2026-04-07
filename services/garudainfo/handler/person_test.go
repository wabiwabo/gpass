package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/garudapass/gpass/services/garudainfo/store"
)

// mockUserDataProvider returns fixed field data for testing.
type mockUserDataProvider struct{}

func (m *mockUserDataProvider) GetUserFields(userID string) map[string]FieldValue {
	return map[string]FieldValue{
		"name":    {Value: "Budi Santoso", Source: "dukcapil", LastVerified: "2025-01-15T10:00:00Z"},
		"dob":     {Value: "1990-05-20", Source: "dukcapil", LastVerified: "2025-01-15T10:00:00Z"},
		"address": {Value: "Jl. Sudirman No. 1, Jakarta", Source: "dukcapil", LastVerified: "2025-01-15T10:00:00Z"},
		"phone":   {Value: "+6281234567890", Source: "user_input", LastVerified: "2025-03-01T12:00:00Z"},
	}
}

func TestGetPerson_FiltersByConsent(t *testing.T) {
	cs := store.NewInMemoryConsentStore()
	provider := &mockUserDataProvider{}
	h := NewPersonHandler(cs, provider)

	// Create consent granting name and dob only (NOT address, NOT phone)
	consent := &store.Consent{
		UserID:          "user-1",
		ClientID:        "client-1",
		ClientName:      "Test App",
		Purpose:         "KYC",
		Fields:          map[string]bool{"name": true, "dob": true, "address": false},
		DurationSeconds: 86400,
	}
	if err := cs.Create(context.Background(), consent); err != nil {
		t.Fatalf("Create consent: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/garudainfo/person?consent_id="+consent.ID, nil)
	rec := httptest.NewRecorder()
	h.GetPerson(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body = %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var resp PersonResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}

	// Should have name and dob
	if _, ok := resp.Fields["name"]; !ok {
		t.Error("expected 'name' in response fields")
	}
	if _, ok := resp.Fields["dob"]; !ok {
		t.Error("expected 'dob' in response fields")
	}

	// Should NOT have address (explicitly false in consent) or phone (not in consent at all)
	if _, ok := resp.Fields["address"]; ok {
		t.Error("'address' should NOT be in response (consent was false)")
	}
	if _, ok := resp.Fields["phone"]; ok {
		t.Error("'phone' should NOT be in response (not in consent)")
	}
}

func TestGetPerson_ConsentNotFound(t *testing.T) {
	cs := store.NewInMemoryConsentStore()
	provider := &mockUserDataProvider{}
	h := NewPersonHandler(cs, provider)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/garudainfo/person?consent_id=nonexistent", nil)
	rec := httptest.NewRecorder()
	h.GetPerson(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestGetPerson_InactiveConsent(t *testing.T) {
	cs := store.NewInMemoryConsentStore()
	provider := &mockUserDataProvider{}
	h := NewPersonHandler(cs, provider)

	consent := &store.Consent{
		UserID:          "user-1",
		ClientID:        "client-1", ClientName: "Test Client", Purpose: "test",
		Fields:          map[string]bool{"name": true},
		DurationSeconds: 86400,
	}
	_ = cs.Create(context.Background(), consent)
	_ = cs.Revoke(context.Background(), consent.ID)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/garudainfo/person?consent_id="+consent.ID, nil)
	rec := httptest.NewRecorder()
	h.GetPerson(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
}
