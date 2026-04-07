package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/garudapass/gpass/services/garudainfo/store"
)

func TestGrant_InvalidJSON(t *testing.T) {
	h := NewConsentHandler(store.NewInMemoryConsentStore())

	req := httptest.NewRequest(http.MethodPost, "/api/v1/garudainfo/consents", bytes.NewBufferString("not json"))
	rec := httptest.NewRecorder()
	h.Grant(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}

func TestGrant_MissingUserID(t *testing.T) {
	h := NewConsentHandler(store.NewInMemoryConsentStore())

	body := `{"client_name":"Test App","purpose":"test","client_id":"c1","fields":["name"],"duration_days":30}`
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString(body))
	rec := httptest.NewRecorder()
	h.Grant(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}

func TestGrant_MissingClientID(t *testing.T) {
	h := NewConsentHandler(store.NewInMemoryConsentStore())

	body := `{"user_id":"u1","fields":["name"],"duration_days":30}`
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString(body))
	rec := httptest.NewRecorder()
	h.Grant(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}

func TestGrant_EmptyFields(t *testing.T) {
	h := NewConsentHandler(store.NewInMemoryConsentStore())

	body := `{"user_id":"u1","client_name":"Test App","purpose":"test","client_id":"c1","fields":[],"duration_days":30}`
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString(body))
	rec := httptest.NewRecorder()
	h.Grant(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for empty fields, got %d", rec.Code)
	}
}

func TestGrant_ZeroDuration(t *testing.T) {
	h := NewConsentHandler(store.NewInMemoryConsentStore())

	body := `{"user_id":"u1","client_name":"Test App","purpose":"test","client_id":"c1","fields":["name"],"duration_days":0}`
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString(body))
	rec := httptest.NewRecorder()
	h.Grant(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for zero duration, got %d", rec.Code)
	}
}

func TestGrant_NegativeDuration(t *testing.T) {
	h := NewConsentHandler(store.NewInMemoryConsentStore())

	body := `{"user_id":"u1","client_name":"Test App","purpose":"test","client_id":"c1","fields":["name"],"duration_days":-1}`
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString(body))
	rec := httptest.NewRecorder()
	h.Grant(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for negative duration, got %d", rec.Code)
	}
}

func TestGrant_MultipleFields(t *testing.T) {
	s := store.NewInMemoryConsentStore()
	h := NewConsentHandler(s)

	body := `{"user_id":"u1","client_id":"c1","client_name":"App","purpose":"KYC","fields":["name","nik","dob","address","phone"],"duration_days":90}`
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString(body))
	rec := httptest.NewRecorder()
	h.Grant(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp grantResponse
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp.ConsentID == "" {
		t.Error("consent_id should be set")
	}
}

func TestGrant_ResponseHeaders(t *testing.T) {
	h := NewConsentHandler(store.NewInMemoryConsentStore())

	body := `{"user_id":"u1","client_name":"Test App","purpose":"test","client_id":"c1","fields":["name"],"duration_days":30}`
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString(body))
	rec := httptest.NewRecorder()
	h.Grant(rec, req)

	if ct := rec.Header().Get("Content-Type"); !strings.Contains(ct, "application/json") {
		t.Errorf("Content-Type: got %q", ct)
	}
	if cc := rec.Header().Get("Cache-Control"); cc != "no-store" {
		t.Errorf("Cache-Control: got %q", cc)
	}
}

func TestList_MissingUserID(t *testing.T) {
	h := NewConsentHandler(store.NewInMemoryConsentStore())

	req := httptest.NewRequest(http.MethodGet, "/api/v1/garudainfo/consents", nil)
	rec := httptest.NewRecorder()
	h.List(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}

func TestList_EmptyResult(t *testing.T) {
	h := NewConsentHandler(store.NewInMemoryConsentStore())

	req := httptest.NewRequest(http.MethodGet, "/api/v1/garudainfo/consents?user_id=nonexistent", nil)
	rec := httptest.NewRecorder()
	h.List(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var resp listResponse
	json.NewDecoder(rec.Body).Decode(&resp)
	if len(resp.Consents) != 0 {
		t.Errorf("expected 0 consents, got %d", len(resp.Consents))
	}
}

func TestRevoke_AlreadyRevoked(t *testing.T) {
	s := store.NewInMemoryConsentStore()
	h := NewConsentHandler(s)

	// Create.
	body := `{"user_id":"u1","client_name":"Test App","purpose":"test","client_id":"c1","fields":["name"],"duration_days":30}`
	createReq := httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString(body))
	createRec := httptest.NewRecorder()
	h.Grant(createRec, createReq)

	var created grantResponse
	json.NewDecoder(createRec.Body).Decode(&created)

	mux := http.NewServeMux()
	mux.HandleFunc("DELETE /consents/{id}", h.Revoke)

	// First revoke — should succeed.
	req1 := httptest.NewRequest(http.MethodDelete, "/consents/"+created.ConsentID, nil)
	rec1 := httptest.NewRecorder()
	mux.ServeHTTP(rec1, req1)
	if rec1.Code != http.StatusOK {
		t.Fatalf("first revoke: expected 200, got %d", rec1.Code)
	}

	// Second revoke — should be conflict.
	req2 := httptest.NewRequest(http.MethodDelete, "/consents/"+created.ConsentID, nil)
	rec2 := httptest.NewRecorder()
	mux.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusConflict {
		t.Errorf("second revoke: expected 409, got %d: %s", rec2.Code, rec2.Body.String())
	}
}

func TestGrant_ThenList_ThenRevoke_Flow(t *testing.T) {
	s := store.NewInMemoryConsentStore()
	h := NewConsentHandler(s)

	// Grant consent.
	grantBody := `{"user_id":"flow-user","client_id":"flow-client","client_name":"FlowApp","purpose":"identity","fields":["name","nik"],"duration_days":365}`
	grantReq := httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString(grantBody))
	grantRec := httptest.NewRecorder()
	h.Grant(grantRec, grantReq)

	if grantRec.Code != http.StatusCreated {
		t.Fatalf("grant: expected 201, got %d", grantRec.Code)
	}

	var granted grantResponse
	json.NewDecoder(grantRec.Body).Decode(&granted)

	// List consents — should see one.
	listReq := httptest.NewRequest(http.MethodGet, "/?user_id=flow-user", nil)
	listRec := httptest.NewRecorder()
	h.List(listRec, listReq)

	var listed listResponse
	json.NewDecoder(listRec.Body).Decode(&listed)
	if len(listed.Consents) != 1 {
		t.Fatalf("list: expected 1 consent, got %d", len(listed.Consents))
	}

	consent := listed.Consents[0]
	if consent.ClientName != "FlowApp" {
		t.Errorf("client_name: got %q", consent.ClientName)
	}
	if consent.Status != "ACTIVE" && consent.Status != "active" {
		t.Errorf("status: got %q, want ACTIVE", consent.Status)
	}
	if !consent.Fields["name"] || !consent.Fields["nik"] {
		t.Errorf("fields: got %v", consent.Fields)
	}

	// Revoke.
	mux := http.NewServeMux()
	mux.HandleFunc("DELETE /c/{id}", h.Revoke)
	revokeReq := httptest.NewRequest(http.MethodDelete, "/c/"+granted.ConsentID, nil)
	revokeRec := httptest.NewRecorder()
	mux.ServeHTTP(revokeRec, revokeReq)

	if revokeRec.Code != http.StatusOK {
		t.Fatalf("revoke: expected 200, got %d", revokeRec.Code)
	}
}

func TestGrant_MultipleConsentsForSameUser(t *testing.T) {
	s := store.NewInMemoryConsentStore()
	h := NewConsentHandler(s)

	for i := 0; i < 3; i++ {
		body := `{"user_id":"multi-user","client_name":"App","purpose":"test","client_id":"client-` + string(rune('A'+i)) + `","fields":["name"],"duration_days":30}`
		req := httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString(body))
		rec := httptest.NewRecorder()
		h.Grant(rec, req)
		if rec.Code != http.StatusCreated {
			t.Fatalf("grant %d: expected 201, got %d", i, rec.Code)
		}
	}

	// List should show all 3.
	listReq := httptest.NewRequest(http.MethodGet, "/?user_id=multi-user", nil)
	listRec := httptest.NewRecorder()
	h.List(listRec, listReq)

	var resp listResponse
	json.NewDecoder(listRec.Body).Decode(&resp)
	if len(resp.Consents) != 3 {
		t.Errorf("expected 3 consents, got %d", len(resp.Consents))
	}
}

func TestList_DifferentUsers(t *testing.T) {
	s := store.NewInMemoryConsentStore()
	h := NewConsentHandler(s)

	// User A gets 2 consents.
	for i := 0; i < 2; i++ {
		body := `{"user_id":"user-a","client_name":"Test App","purpose":"test","client_id":"c1","fields":["name"],"duration_days":30}`
		req := httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString(body))
		h.Grant(httptest.NewRecorder(), req)
	}

	// User B gets 1 consent.
	body := `{"user_id":"user-b","client_name":"Test App","purpose":"test","client_id":"c1","fields":["name"],"duration_days":30}`
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString(body))
	h.Grant(httptest.NewRecorder(), req)

	// List for user A.
	listReq := httptest.NewRequest(http.MethodGet, "/?user_id=user-a", nil)
	listRec := httptest.NewRecorder()
	h.List(listRec, listReq)

	var resp listResponse
	json.NewDecoder(listRec.Body).Decode(&resp)
	if len(resp.Consents) != 2 {
		t.Errorf("user-a: expected 2, got %d", len(resp.Consents))
	}
}

func TestGrant_EmptyBody(t *testing.T) {
	h := NewConsentHandler(store.NewInMemoryConsentStore())

	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString(""))
	rec := httptest.NewRecorder()
	h.Grant(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for empty body, got %d", rec.Code)
	}
}

func TestGrant_ErrorResponseFormat(t *testing.T) {
	h := NewConsentHandler(store.NewInMemoryConsentStore())

	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString("{}"))
	rec := httptest.NewRecorder()
	h.Grant(rec, req)

	var resp errorResponse
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp.Error == "" {
		t.Error("error code should be set")
	}
	if resp.Message == "" {
		t.Error("message should be set")
	}
}
