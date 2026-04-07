package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/garudapass/gpass/services/garudainfo/store"
)

func newConsentHandler() *ConsentHandler {
	return NewConsentHandler(store.NewInMemoryConsentStore())
}

func TestGrantValidation(t *testing.T) {
	tests := []struct {
		name       string
		body       string
		wantStatus int
		wantError  string
	}{
		{
			"missing_user_id",
			`{"client_name":"Test App","purpose":"test","client_id":"c1","fields":["name"],"duration_days":30}`,
			400, "invalid_request",
		},
		{
			"missing_client_id",
			`{"user_id":"u1","fields":["name"],"duration_days":30}`,
			400, "invalid_request",
		},
		{
			"missing_fields",
			`{"user_id":"u1","client_name":"Test App","purpose":"test","client_id":"c1","duration_days":30}`,
			400, "invalid_request",
		},
		{
			"empty_fields",
			`{"user_id":"u1","client_name":"Test App","purpose":"test","client_id":"c1","fields":[],"duration_days":30}`,
			400, "invalid_request",
		},
		{
			"zero_duration",
			`{"user_id":"u1","client_name":"Test App","purpose":"test","client_id":"c1","fields":["name"],"duration_days":0}`,
			400, "invalid_request",
		},
		{
			"negative_duration",
			`{"user_id":"u1","client_name":"Test App","purpose":"test","client_id":"c1","fields":["name"],"duration_days":-1}`,
			400, "invalid_request",
		},
		{
			"invalid_json",
			`{not json`,
			400, "invalid_request",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newConsentHandler()
			req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(tt.body))
			rr := httptest.NewRecorder()
			h.Grant(rr, req)

			if rr.Code != tt.wantStatus {
				t.Errorf("status: got %d, want %d", rr.Code, tt.wantStatus)
			}
			var resp errorResponse
			json.NewDecoder(rr.Body).Decode(&resp)
			if resp.Error != tt.wantError {
				t.Errorf("error: got %q, want %q", resp.Error, tt.wantError)
			}
		})
	}
}

func TestGrantSuccess(t *testing.T) {
	h := newConsentHandler()
	body := `{"user_id":"u1","client_id":"c1","client_name":"Test App","purpose":"kyc","fields":["name","email","nik"],"duration_days":90}`
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	rr := httptest.NewRecorder()
	h.Grant(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("status: got %d, want 201", rr.Code)
	}

	var resp grantResponse
	json.NewDecoder(rr.Body).Decode(&resp)
	if resp.ConsentID == "" {
		t.Error("consent_id should be set")
	}
	if resp.ExpiresAt == "" {
		t.Error("expires_at should be set")
	}
}

func TestGrantDurationCalculation(t *testing.T) {
	h := newConsentHandler()
	body := `{"user_id":"u1","client_name":"Test App","purpose":"test","client_id":"c1","fields":["name"],"duration_days":365}`
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	rr := httptest.NewRecorder()
	h.Grant(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("status: got %d", rr.Code)
	}
	var resp grantResponse
	json.NewDecoder(rr.Body).Decode(&resp)
	if resp.ExpiresAt == "" {
		t.Fatal("expires_at should be set")
	}
}

func TestGrantResponseHeaders(t *testing.T) {
	h := newConsentHandler()
	body := `{"user_id":"u1","client_name":"Test App","purpose":"test","client_id":"c1","fields":["name"],"duration_days":30}`
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	rr := httptest.NewRecorder()
	h.Grant(rr, req)

	ct := rr.Header().Get("Content-Type")
	if ct != "application/json; charset=utf-8" {
		t.Errorf("Content-Type: got %q", ct)
	}
	cc := rr.Header().Get("Cache-Control")
	if cc != "no-store" {
		t.Errorf("Cache-Control: got %q", cc)
	}
}

func TestListValidation(t *testing.T) {
	h := newConsentHandler()

	// Missing user_id
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	h.List(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want 400", rr.Code)
	}
}

func TestListSuccess(t *testing.T) {
	h := newConsentHandler()

	// Grant two consents
	for _, body := range []string{
		`{"user_id":"u1","client_name":"Test App","purpose":"test","client_id":"c1","fields":["name"],"duration_days":30}`,
		`{"user_id":"u1","client_name":"Test App","purpose":"test","client_id":"c2","fields":["email"],"duration_days":60}`,
	} {
		req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
		rr := httptest.NewRecorder()
		h.Grant(rr, req)
		if rr.Code != http.StatusCreated {
			t.Fatalf("grant failed: %d", rr.Code)
		}
	}

	// List
	req := httptest.NewRequest(http.MethodGet, "/?user_id=u1", nil)
	rr := httptest.NewRecorder()
	h.List(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status: got %d", rr.Code)
	}
	var resp listResponse
	json.NewDecoder(rr.Body).Decode(&resp)
	if len(resp.Consents) != 2 {
		t.Errorf("consents: got %d, want 2", len(resp.Consents))
	}
}

func TestListEmpty(t *testing.T) {
	h := newConsentHandler()
	req := httptest.NewRequest(http.MethodGet, "/?user_id=nobody", nil)
	rr := httptest.NewRecorder()
	h.List(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status: got %d", rr.Code)
	}
	var resp listResponse
	json.NewDecoder(rr.Body).Decode(&resp)
	if len(resp.Consents) != 0 {
		t.Errorf("should be empty, got %d", len(resp.Consents))
	}
}

func TestListConsentDTO(t *testing.T) {
	h := newConsentHandler()

	body := `{"user_id":"u1","client_id":"c1","client_name":"My App","purpose":"kyc","fields":["name","email"],"duration_days":30}`
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	rr := httptest.NewRecorder()
	h.Grant(rr, req)

	req = httptest.NewRequest(http.MethodGet, "/?user_id=u1", nil)
	rr = httptest.NewRecorder()
	h.List(rr, req)

	var resp listResponse
	json.NewDecoder(rr.Body).Decode(&resp)

	dto := resp.Consents[0]
	if dto.UserID != "u1" {
		t.Errorf("UserID: %q", dto.UserID)
	}
	if dto.ClientID != "c1" {
		t.Errorf("ClientID: %q", dto.ClientID)
	}
	if dto.ClientName != "My App" {
		t.Errorf("ClientName: %q", dto.ClientName)
	}
	if dto.Purpose != "kyc" {
		t.Errorf("Purpose: %q", dto.Purpose)
	}
	if dto.Status != "ACTIVE" {
		t.Errorf("Status: %q", dto.Status)
	}
	if !dto.Fields["name"] || !dto.Fields["email"] {
		t.Errorf("Fields: %v", dto.Fields)
	}
	if dto.GrantedAt == "" {
		t.Error("GrantedAt should be set")
	}
	if dto.ExpiresAt == "" {
		t.Error("ExpiresAt should be set")
	}
}

func TestRevokeNotFound(t *testing.T) {
	h := newConsentHandler()
	req := httptest.NewRequest(http.MethodDelete, "/", nil)
	req.SetPathValue("id", "nonexistent-id")
	rr := httptest.NewRecorder()
	h.Revoke(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("status: got %d, want 404", rr.Code)
	}
}

func TestRevokeMissingID(t *testing.T) {
	h := newConsentHandler()
	req := httptest.NewRequest(http.MethodDelete, "/", nil)
	rr := httptest.NewRecorder()
	h.Revoke(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want 400", rr.Code)
	}
}

func TestRevokeAlreadyRevoked(t *testing.T) {
	h := newConsentHandler()

	// Grant
	body := `{"user_id":"u1","client_name":"Test App","purpose":"test","client_id":"c1","fields":["name"],"duration_days":30}`
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	rr := httptest.NewRecorder()
	h.Grant(rr, req)
	var gResp grantResponse
	json.NewDecoder(rr.Body).Decode(&gResp)

	// First revoke
	req = httptest.NewRequest(http.MethodDelete, "/", nil)
	req.SetPathValue("id", gResp.ConsentID)
	rr = httptest.NewRecorder()
	h.Revoke(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("first revoke: got %d", rr.Code)
	}

	// Second revoke
	req = httptest.NewRequest(http.MethodDelete, "/", nil)
	req.SetPathValue("id", gResp.ConsentID)
	rr = httptest.NewRecorder()
	h.Revoke(rr, req)
	if rr.Code != http.StatusConflict {
		t.Errorf("second revoke: got %d, want 409", rr.Code)
	}
}

func TestGrantListRevokeFlow(t *testing.T) {
	h := newConsentHandler()

	// Grant
	body := `{"user_id":"u1","client_name":"Test App","purpose":"test","client_id":"c1","fields":["name","email"],"duration_days":90}`
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	rr := httptest.NewRecorder()
	h.Grant(rr, req)
	if rr.Code != 201 {
		t.Fatalf("grant: %d", rr.Code)
	}
	var gResp grantResponse
	json.NewDecoder(rr.Body).Decode(&gResp)

	// List — should have 1 active
	req = httptest.NewRequest(http.MethodGet, "/?user_id=u1", nil)
	rr = httptest.NewRecorder()
	h.List(rr, req)
	var lResp listResponse
	json.NewDecoder(rr.Body).Decode(&lResp)
	if len(lResp.Consents) != 1 {
		t.Fatalf("list: got %d", len(lResp.Consents))
	}
	if lResp.Consents[0].Status != "ACTIVE" {
		t.Errorf("status: %q", lResp.Consents[0].Status)
	}

	// Revoke
	req = httptest.NewRequest(http.MethodDelete, "/", nil)
	req.SetPathValue("id", gResp.ConsentID)
	rr = httptest.NewRecorder()
	h.Revoke(rr, req)
	if rr.Code != 200 {
		t.Fatalf("revoke: %d", rr.Code)
	}

	// List again — should show REVOKED
	req = httptest.NewRequest(http.MethodGet, "/?user_id=u1", nil)
	rr = httptest.NewRecorder()
	h.List(rr, req)
	json.NewDecoder(rr.Body).Decode(&lResp)
	if lResp.Consents[0].Status != "REVOKED" {
		t.Errorf("after revoke status: %q", lResp.Consents[0].Status)
	}
}

func TestGrantMultipleConsentsForSameUser(t *testing.T) {
	h := newConsentHandler()

	for i := 0; i < 5; i++ {
		body := `{"user_id":"u1","client_name":"Test App","purpose":"test","client_id":"c1","fields":["name"],"duration_days":30}`
		req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
		rr := httptest.NewRecorder()
		h.Grant(rr, req)
		if rr.Code != 201 {
			t.Fatalf("grant %d: %d", i, rr.Code)
		}
	}

	req := httptest.NewRequest(http.MethodGet, "/?user_id=u1", nil)
	rr := httptest.NewRecorder()
	h.List(rr, req)
	var resp listResponse
	json.NewDecoder(rr.Body).Decode(&resp)
	if len(resp.Consents) != 5 {
		t.Errorf("got %d, want 5", len(resp.Consents))
	}
}

func TestGrantIsolatesDifferentUsers(t *testing.T) {
	h := newConsentHandler()

	body1 := `{"user_id":"alice","client_name":"Test App","purpose":"test","client_id":"c1","fields":["name"],"duration_days":30}`
	body2 := `{"user_id":"bob","client_name":"Test App","purpose":"test","client_id":"c1","fields":["email"],"duration_days":30}`

	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body1))
	rr := httptest.NewRecorder()
	h.Grant(rr, req)

	req = httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body2))
	rr = httptest.NewRecorder()
	h.Grant(rr, req)

	// Alice should only see her consent
	req = httptest.NewRequest(http.MethodGet, "/?user_id=alice", nil)
	rr = httptest.NewRecorder()
	h.List(rr, req)
	var resp listResponse
	json.NewDecoder(rr.Body).Decode(&resp)
	if len(resp.Consents) != 1 {
		t.Errorf("alice: got %d, want 1", len(resp.Consents))
	}
	if resp.Consents[0].UserID != "alice" {
		t.Error("should be alice's consent")
	}
}
