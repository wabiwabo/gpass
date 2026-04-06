package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/garudapass/gpass/services/garudainfo/store"
)

func TestGrant(t *testing.T) {
	s := store.NewInMemoryConsentStore()
	h := NewConsentHandler(s)

	body := `{"user_id":"u1","client_id":"c1","client_name":"App","purpose":"KYC","fields":["name","dob"],"duration_days":30}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/garudainfo/consents", bytes.NewBufferString(body))
	rec := httptest.NewRecorder()

	h.Grant(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusCreated)
	}

	var resp grantResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.ConsentID == "" {
		t.Error("consent_id should not be empty")
	}
	if resp.ExpiresAt == "" {
		t.Error("expires_at should not be empty")
	}

	ct := rec.Header().Get("Content-Type")
	if ct != "application/json; charset=utf-8" {
		t.Errorf("Content-Type = %q, want application/json; charset=utf-8", ct)
	}
}

func TestList(t *testing.T) {
	s := store.NewInMemoryConsentStore()
	h := NewConsentHandler(s)

	// Create two consents
	for range 2 {
		body := `{"user_id":"u1","client_id":"c1","client_name":"App","purpose":"KYC","fields":["name"],"duration_days":30}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/garudainfo/consents", bytes.NewBufferString(body))
		rec := httptest.NewRecorder()
		h.Grant(rec, req)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/garudainfo/consents?user_id=u1", nil)
	rec := httptest.NewRecorder()
	h.List(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var resp listResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(resp.Consents) != 2 {
		t.Errorf("got %d consents, want 2", len(resp.Consents))
	}
}

func TestRevoke(t *testing.T) {
	s := store.NewInMemoryConsentStore()
	h := NewConsentHandler(s)

	// Create a consent
	body := `{"user_id":"u1","client_id":"c1","client_name":"App","purpose":"KYC","fields":["name"],"duration_days":30}`
	createReq := httptest.NewRequest(http.MethodPost, "/api/v1/garudainfo/consents", bytes.NewBufferString(body))
	createRec := httptest.NewRecorder()
	h.Grant(createRec, createReq)

	var created grantResponse
	_ = json.NewDecoder(createRec.Body).Decode(&created)

	// Revoke it using a mux to get PathValue working
	mux := http.NewServeMux()
	mux.HandleFunc("DELETE /api/v1/garudainfo/consents/{id}", h.Revoke)

	revokeReq := httptest.NewRequest(http.MethodDelete, "/api/v1/garudainfo/consents/"+created.ConsentID, nil)
	revokeRec := httptest.NewRecorder()
	mux.ServeHTTP(revokeRec, revokeReq)

	if revokeRec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body = %s", revokeRec.Code, http.StatusOK, revokeRec.Body.String())
	}

	var resp revokeResponse
	_ = json.NewDecoder(revokeRec.Body).Decode(&resp)
	if !resp.Revoked {
		t.Error("revoked should be true")
	}
}

func TestRevoke_NotFound(t *testing.T) {
	s := store.NewInMemoryConsentStore()
	h := NewConsentHandler(s)

	mux := http.NewServeMux()
	mux.HandleFunc("DELETE /api/v1/garudainfo/consents/{id}", h.Revoke)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/garudainfo/consents/nonexistent", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}
