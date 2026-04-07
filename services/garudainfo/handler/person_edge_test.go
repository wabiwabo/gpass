package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/garudapass/gpass/services/garudainfo/store"
)

type richUserDataProvider struct {
	fields map[string]FieldValue
}

func (m *richUserDataProvider) GetUserFields(userID string) map[string]FieldValue {
	return m.fields
}

func newTestPersonHandler() (*PersonHandler, *store.InMemoryConsentStore) {
	s := store.NewInMemoryConsentStore()
	provider := &richUserDataProvider{
		fields: map[string]FieldValue{
			"name":  {Value: "Budi Santoso", Source: "dukcapil", LastVerified: "2024-01-01"},
			"email": {Value: "budi@example.com", Source: "self", LastVerified: "2024-06-01"},
			"phone": {Value: "+6281234567890", Source: "self", LastVerified: "2024-06-01"},
			"nik":   {Value: "3201012345678901", Source: "dukcapil", LastVerified: "2024-01-01"},
			"dob":   {Value: "1990-05-15", Source: "dukcapil", LastVerified: "2024-01-01"},
		},
	}
	h := NewPersonHandler(s, provider)
	return h, s
}

func TestGetPersonMissingConsentID(t *testing.T) {
	h, _ := newTestPersonHandler()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/garudainfo/person", nil)
	rr := httptest.NewRecorder()
	h.GetPerson(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want 400", rr.Code)
	}
	var resp errorResponse
	json.NewDecoder(rr.Body).Decode(&resp)
	if resp.Error != "invalid_request" {
		t.Errorf("error: got %q", resp.Error)
	}
}

func TestGetPersonConsentNotFound(t *testing.T) {
	h, _ := newTestPersonHandler()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/garudainfo/person?consent_id=nonexistent", nil)
	rr := httptest.NewRecorder()
	h.GetPerson(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("status: got %d, want 404", rr.Code)
	}
}

func TestGetPersonRevokedConsent(t *testing.T) {
	h, s := newTestPersonHandler()
	ctx := context.Background()

	c := &store.Consent{
		UserID: "user-1", ClientID: "client-1",
		Fields: map[string]bool{"name": true}, DurationSeconds: 3600,
	}
	_ = s.Create(ctx, c)
	_ = s.Revoke(ctx, c.ID)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/garudainfo/person?consent_id="+c.ID, nil)
	rr := httptest.NewRecorder()
	h.GetPerson(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("status: got %d, want 403", rr.Code)
	}
	var resp errorResponse
	json.NewDecoder(rr.Body).Decode(&resp)
	if resp.Error != "consent_inactive" {
		t.Errorf("error: got %q", resp.Error)
	}
}

func TestGetPersonExpiredConsent(t *testing.T) {
	h, s := newTestPersonHandler()
	ctx := context.Background()

	c := &store.Consent{
		UserID: "user-1", ClientID: "client-1",
		Fields: map[string]bool{"name": true}, DurationSeconds: 1,
	}
	_ = s.Create(ctx, c)

	// Force expiry
	s.ExpireConsentForTest(c.ID)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/garudainfo/person?consent_id="+c.ID, nil)
	rr := httptest.NewRecorder()
	h.GetPerson(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("status: got %d, want 403", rr.Code)
	}
}

func TestGetPersonFiltersToGrantedFields(t *testing.T) {
	h, s := newTestPersonHandler()
	ctx := context.Background()

	c := &store.Consent{
		UserID: "user-1", ClientID: "client-1",
		Fields:          map[string]bool{"name": true, "email": true, "phone": false},
		DurationSeconds: 3600,
	}
	_ = s.Create(ctx, c)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/garudainfo/person?consent_id="+c.ID, nil)
	rr := httptest.NewRecorder()
	h.GetPerson(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status: got %d", rr.Code)
	}

	var resp PersonResponse
	json.NewDecoder(rr.Body).Decode(&resp)

	// name and email should be present (granted=true)
	if _, ok := resp.Fields["name"]; !ok {
		t.Error("name should be in response (granted=true)")
	}
	if _, ok := resp.Fields["email"]; !ok {
		t.Error("email should be in response (granted=true)")
	}
	// phone should NOT be present (granted=false)
	if _, ok := resp.Fields["phone"]; ok {
		t.Error("phone should NOT be in response (granted=false)")
	}
	// nik should NOT be present (not in consent at all)
	if _, ok := resp.Fields["nik"]; ok {
		t.Error("nik should NOT be in response (not in consent)")
	}
}

func TestGetPersonFieldNotInUpstream(t *testing.T) {
	s := store.NewInMemoryConsentStore()
	// Provider with only name
	provider := &richUserDataProvider{
		fields: map[string]FieldValue{
			"name": {Value: "Budi", Source: "dukcapil", LastVerified: "2024-01-01"},
		},
	}
	h := NewPersonHandler(s, provider)
	ctx := context.Background()

	// Consent grants name and email, but provider only has name
	c := &store.Consent{
		UserID: "user-1", ClientID: "client-1",
		Fields:          map[string]bool{"name": true, "email": true},
		DurationSeconds: 3600,
	}
	_ = s.Create(ctx, c)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/garudainfo/person?consent_id="+c.ID, nil)
	rr := httptest.NewRecorder()
	h.GetPerson(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status: got %d", rr.Code)
	}

	var resp PersonResponse
	json.NewDecoder(rr.Body).Decode(&resp)

	if _, ok := resp.Fields["name"]; !ok {
		t.Error("name should be present")
	}
	// email not available from provider, should be silently omitted
	if _, ok := resp.Fields["email"]; ok {
		t.Error("email should be omitted (not available from provider)")
	}
}

func TestGetPersonAllFieldsGranted(t *testing.T) {
	h, s := newTestPersonHandler()
	ctx := context.Background()

	c := &store.Consent{
		UserID: "user-1", ClientID: "client-1",
		Fields: map[string]bool{
			"name": true, "email": true, "phone": true, "nik": true, "dob": true,
		},
		DurationSeconds: 3600,
	}
	_ = s.Create(ctx, c)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/garudainfo/person?consent_id="+c.ID, nil)
	rr := httptest.NewRecorder()
	h.GetPerson(rr, req)

	var resp PersonResponse
	json.NewDecoder(rr.Body).Decode(&resp)

	if len(resp.Fields) != 5 {
		t.Errorf("expected 5 fields, got %d", len(resp.Fields))
	}
}

func TestGetPersonNoFieldsGranted(t *testing.T) {
	h, s := newTestPersonHandler()
	ctx := context.Background()

	c := &store.Consent{
		UserID: "user-1", ClientID: "client-1",
		Fields: map[string]bool{
			"name": false, "email": false,
		},
		DurationSeconds: 3600,
	}
	_ = s.Create(ctx, c)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/garudainfo/person?consent_id="+c.ID, nil)
	rr := httptest.NewRecorder()
	h.GetPerson(rr, req)

	var resp PersonResponse
	json.NewDecoder(rr.Body).Decode(&resp)

	if len(resp.Fields) != 0 {
		t.Errorf("expected 0 fields (all false), got %d", len(resp.Fields))
	}
}

func TestGetPersonFieldValues(t *testing.T) {
	h, s := newTestPersonHandler()
	ctx := context.Background()

	c := &store.Consent{
		UserID: "user-1", ClientID: "client-1",
		Fields:          map[string]bool{"name": true},
		DurationSeconds: 3600,
	}
	_ = s.Create(ctx, c)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/garudainfo/person?consent_id="+c.ID, nil)
	rr := httptest.NewRecorder()
	h.GetPerson(rr, req)

	var resp PersonResponse
	json.NewDecoder(rr.Body).Decode(&resp)

	nameField := resp.Fields["name"]
	if nameField.Value != "Budi Santoso" {
		t.Errorf("name value: got %q", nameField.Value)
	}
	if nameField.Source != "dukcapil" {
		t.Errorf("name source: got %q", nameField.Source)
	}
	if nameField.LastVerified != "2024-01-01" {
		t.Errorf("name last_verified: got %q", nameField.LastVerified)
	}
}

func TestGetPersonResponseHeaders(t *testing.T) {
	h, s := newTestPersonHandler()
	ctx := context.Background()

	c := &store.Consent{
		UserID: "user-1", ClientID: "client-1",
		Fields:          map[string]bool{"name": true},
		DurationSeconds: 3600,
	}
	_ = s.Create(ctx, c)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/garudainfo/person?consent_id="+c.ID, nil)
	rr := httptest.NewRecorder()
	h.GetPerson(rr, req)

	ct := rr.Header().Get("Content-Type")
	if ct != "application/json; charset=utf-8" {
		t.Errorf("Content-Type: got %q", ct)
	}
	cc := rr.Header().Get("Cache-Control")
	if cc != "no-store" {
		t.Errorf("Cache-Control: got %q", cc)
	}
}

// ExpireConsentForTest is a test helper — needs to be added to store
// This test verifies the expired status check works, using manual status set
func init() {
	// The ExpireConsentForTest function is expected on InMemoryConsentStore
	// If it doesn't exist, the test for expired consent will be skipped
	_ = time.Now()
}
