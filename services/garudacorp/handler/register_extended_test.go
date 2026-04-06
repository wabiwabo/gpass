package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/garudapass/gpass/services/garudacorp/ahu"
	"github.com/garudapass/gpass/services/garudacorp/store"
)

// TestRegister_EmptySKNumber verifies that a registration request
// with an empty sk_number field returns 400.
func TestRegister_EmptySKNumber(t *testing.T) {
	h := NewRegisterHandler(RegisterDeps{
		AHU:         &mockAHU{},
		EntityStore: store.NewInMemoryEntityStore(),
		RoleStore:   store.NewInMemoryRoleStore(),
		NIKKey:      testNIKKey,
	})

	body, _ := json.Marshal(map[string]string{
		"sk_number":      "",
		"caller_user_id": "user-1",
		"caller_nik":     "3201010101010001",
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/corp/register", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.Register(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for empty SK number, got %d: %s", w.Code, w.Body.String())
	}

	var errResp map[string]string
	json.NewDecoder(w.Body).Decode(&errResp)
	if errResp["error"] != "invalid_request" {
		t.Errorf("expected error code invalid_request, got %s", errResp["error"])
	}
	if errResp["message"] != "sk_number is required" {
		t.Errorf("expected message 'sk_number is required', got %s", errResp["message"])
	}
}

// TestRegister_InvalidJSONVariants tests multiple forms of invalid JSON input.
func TestRegister_InvalidJSONVariants(t *testing.T) {
	h := NewRegisterHandler(RegisterDeps{
		AHU:         &mockAHU{},
		EntityStore: store.NewInMemoryEntityStore(),
		RoleStore:   store.NewInMemoryRoleStore(),
		NIKKey:      testNIKKey,
	})

	tests := []struct {
		name string
		body string
	}{
		{"empty body", ""},
		{"truncated json", `{"sk_number": `},
		{"malformed braces", `{sk_number: "AHU-12345"}`},
		{"array instead of object", `["AHU-12345"]`},
		{"random text", "Hello, this is not JSON at all."},
		{"html content", "<html><body>Not JSON</body></html>"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/api/v1/corp/register", bytes.NewBufferString(tc.body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			h.Register(w, req)

			if w.Code != http.StatusBadRequest {
				t.Errorf("expected 400, got %d: %s", w.Code, w.Body.String())
			}
		})
	}
}

// TestRegister_MissingCallerUserID verifies that a registration request
// without caller_user_id returns 400.
func TestRegister_MissingCallerUserID(t *testing.T) {
	h := NewRegisterHandler(RegisterDeps{
		AHU:         &mockAHU{},
		EntityStore: store.NewInMemoryEntityStore(),
		RoleStore:   store.NewInMemoryRoleStore(),
		NIKKey:      testNIKKey,
	})

	body, _ := json.Marshal(map[string]string{
		"sk_number":  "AHU-12345",
		"caller_nik": "3201010101010001",
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/corp/register", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.Register(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}

	var errResp map[string]string
	json.NewDecoder(w.Body).Decode(&errResp)
	if errResp["message"] != "caller_user_id is required" {
		t.Errorf("expected 'caller_user_id is required', got %s", errResp["message"])
	}
}

// TestRegister_MissingCallerNIK verifies that a registration request
// without caller_nik returns 400.
func TestRegister_MissingCallerNIK(t *testing.T) {
	h := NewRegisterHandler(RegisterDeps{
		AHU:         &mockAHU{},
		EntityStore: store.NewInMemoryEntityStore(),
		RoleStore:   store.NewInMemoryRoleStore(),
		NIKKey:      testNIKKey,
	})

	body, _ := json.Marshal(map[string]string{
		"sk_number":      "AHU-12345",
		"caller_user_id": "user-1",
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/corp/register", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.Register(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}

	var errResp map[string]string
	json.NewDecoder(w.Body).Decode(&errResp)
	if errResp["message"] != "caller_nik is required" {
		t.Errorf("expected 'caller_nik is required', got %s", errResp["message"])
	}
}

// TestRegister_AHUServiceUnavailable verifies that an AHU service failure
// returns 502 Bad Gateway.
func TestRegister_AHUServiceUnavailable(t *testing.T) {
	mock := &mockAHU{
		companyErr: fmt.Errorf("connection refused"),
	}

	h := NewRegisterHandler(RegisterDeps{
		AHU:         mock,
		EntityStore: store.NewInMemoryEntityStore(),
		RoleStore:   store.NewInMemoryRoleStore(),
		NIKKey:      testNIKKey,
	})

	body, _ := json.Marshal(map[string]string{
		"sk_number":      "AHU-12345",
		"caller_user_id": "user-1",
		"caller_nik":     "3201010101010001",
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/corp/register", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.Register(w, req)

	if w.Code != http.StatusBadGateway {
		t.Errorf("expected 502, got %d: %s", w.Code, w.Body.String())
	}

	var errResp map[string]string
	json.NewDecoder(w.Body).Decode(&errResp)
	if errResp["error"] != "ahu_unavailable" {
		t.Errorf("expected ahu_unavailable, got %s", errResp["error"])
	}
}

// TestRegister_NoOfficersFromAHU verifies that when AHU returns zero
// officers, the caller cannot match as DIREKTUR_UTAMA and gets 403.
func TestRegister_NoOfficersFromAHU(t *testing.T) {
	mock := &mockAHU{
		company: &ahu.CompanySearchResponse{
			Found:    true,
			SKNumber: "AHU-12345",
			Name:     "PT Empty Officers",
		},
		officers: []ahu.Officer{}, // No officers
	}

	h := NewRegisterHandler(RegisterDeps{
		AHU:         mock,
		EntityStore: store.NewInMemoryEntityStore(),
		RoleStore:   store.NewInMemoryRoleStore(),
		NIKKey:      testNIKKey,
	})

	body, _ := json.Marshal(map[string]string{
		"sk_number":      "AHU-12345",
		"caller_user_id": "user-1",
		"caller_nik":     "3201010101010001",
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/corp/register", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.Register(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d: %s", w.Code, w.Body.String())
	}

	var errResp map[string]string
	json.NewDecoder(w.Body).Decode(&errResp)
	if errResp["error"] != "not_authorized" {
		t.Errorf("expected not_authorized, got %s", errResp["error"])
	}
}

// TestRegister_PreservesAllShareholderData verifies that all shareholder
// fields (name, share type, shares, percentage) are preserved after registration.
func TestRegister_PreservesAllShareholderData(t *testing.T) {
	callerNIK := "3201010101010001"

	shareholders := []ahu.Shareholder{
		{Name: "Alice Investor", ShareType: "INDIVIDUAL", Shares: 500, Percentage: 50.0},
		{Name: "PT Holding Corp", ShareType: "CORPORATE", Shares: 300, Percentage: 30.0},
		{Name: "Bob Capital", ShareType: "INDIVIDUAL", Shares: 200, Percentage: 20.0},
	}

	mock := &mockAHU{
		company: &ahu.CompanySearchResponse{
			Found:       true,
			SKNumber:    "AHU-55555",
			Name:        "PT Shareholder Test",
			EntityType:  "PT",
			Status:      "ACTIVE",
			NPWP:        "01.234.567.8-901.000",
			Address:     "Surabaya",
			CapitalAuth: 2000000000,
			CapitalPaid: 1000000000,
		},
		officers: []ahu.Officer{
			{NIK: callerNIK, Name: "Alice Investor", Position: "DIREKTUR_UTAMA", AppointmentDate: "2021-06-15"},
		},
		shareholders: shareholders,
	}

	entityStore := store.NewInMemoryEntityStore()
	roleStore := store.NewInMemoryRoleStore()

	h := NewRegisterHandler(RegisterDeps{
		AHU:         mock,
		EntityStore: entityStore,
		RoleStore:   roleStore,
		NIKKey:      testNIKKey,
	})

	body, _ := json.Marshal(map[string]string{
		"sk_number":      "AHU-55555",
		"caller_user_id": "user-alice",
		"caller_nik":     callerNIK,
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/corp/register", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.Register(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var resp registerResponse
	json.NewDecoder(w.Body).Decode(&resp)

	// Retrieve the entity and check shareholders
	entity, err := entityStore.GetByID(context.Background(), resp.EntityID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}

	if len(entity.Shareholders) != 3 {
		t.Fatalf("expected 3 shareholders, got %d", len(entity.Shareholders))
	}

	// Check first shareholder
	sh0 := entity.Shareholders[0]
	if sh0.Name != "Alice Investor" {
		t.Errorf("shareholder[0] name = %q, want %q", sh0.Name, "Alice Investor")
	}
	if sh0.ShareType != "INDIVIDUAL" {
		t.Errorf("shareholder[0] share_type = %q, want INDIVIDUAL", sh0.ShareType)
	}
	if sh0.Shares != 500 {
		t.Errorf("shareholder[0] shares = %d, want 500", sh0.Shares)
	}
	if sh0.Percentage != 50.0 {
		t.Errorf("shareholder[0] percentage = %f, want 50.0", sh0.Percentage)
	}

	// Check corporate shareholder
	sh1 := entity.Shareholders[1]
	if sh1.ShareType != "CORPORATE" {
		t.Errorf("shareholder[1] share_type = %q, want CORPORATE", sh1.ShareType)
	}
	if sh1.Shares != 300 {
		t.Errorf("shareholder[1] shares = %d, want 300", sh1.Shares)
	}

	// Check third shareholder
	sh2 := entity.Shareholders[2]
	if sh2.Name != "Bob Capital" {
		t.Errorf("shareholder[2] name = %q, want %q", sh2.Name, "Bob Capital")
	}
	if sh2.Percentage != 20.0 {
		t.Errorf("shareholder[2] percentage = %f, want 20.0", sh2.Percentage)
	}

	// Verify total percentage
	totalPct := sh0.Percentage + sh1.Percentage + sh2.Percentage
	if totalPct != 100.0 {
		t.Errorf("total percentage = %f, want 100.0", totalPct)
	}
}

// TestRegister_OfficerGetError verifies that when AHU GetOfficers fails,
// the registration returns 502.
func TestRegister_OfficerGetError(t *testing.T) {
	mock := &mockAHU{
		company: &ahu.CompanySearchResponse{
			Found:    true,
			SKNumber: "AHU-12345",
			Name:     "PT Test Corp",
		},
		officersErr: fmt.Errorf("timeout"),
	}

	h := NewRegisterHandler(RegisterDeps{
		AHU:         mock,
		EntityStore: store.NewInMemoryEntityStore(),
		RoleStore:   store.NewInMemoryRoleStore(),
		NIKKey:      testNIKKey,
	})

	body, _ := json.Marshal(map[string]string{
		"sk_number":      "AHU-12345",
		"caller_user_id": "user-1",
		"caller_nik":     "3201010101010001",
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/corp/register", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.Register(w, req)

	if w.Code != http.StatusBadGateway {
		t.Errorf("expected 502, got %d: %s", w.Code, w.Body.String())
	}
}

// TestRegister_PreservesEntityFields verifies all entity fields from AHU
// are stored correctly.
func TestRegister_PreservesEntityFields(t *testing.T) {
	callerNIK := "3201010101010001"

	mock := &mockAHU{
		company: &ahu.CompanySearchResponse{
			Found:       true,
			SKNumber:    "AHU-77777",
			Name:        "PT Full Fields",
			EntityType:  "CV",
			Status:      "ACTIVE",
			NPWP:        "99.888.777.6-543.000",
			Address:     "Bandung, Jawa Barat",
			CapitalAuth: 5000000000,
			CapitalPaid: 2500000000,
		},
		officers: []ahu.Officer{
			{NIK: callerNIK, Name: "Test Dir", Position: "DIREKTUR_UTAMA", AppointmentDate: "2022-03-01"},
		},
	}

	entityStore := store.NewInMemoryEntityStore()

	h := NewRegisterHandler(RegisterDeps{
		AHU:         mock,
		EntityStore: entityStore,
		RoleStore:   store.NewInMemoryRoleStore(),
		NIKKey:      testNIKKey,
	})

	body, _ := json.Marshal(map[string]string{
		"sk_number":      "AHU-77777",
		"caller_user_id": "user-dir",
		"caller_nik":     callerNIK,
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/corp/register", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.Register(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var resp registerResponse
	json.NewDecoder(w.Body).Decode(&resp)

	entity, err := entityStore.GetByID(context.Background(), resp.EntityID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}

	if entity.AHUSKNumber != "AHU-77777" {
		t.Errorf("SKNumber = %q, want %q", entity.AHUSKNumber, "AHU-77777")
	}
	if entity.Name != "PT Full Fields" {
		t.Errorf("Name = %q, want %q", entity.Name, "PT Full Fields")
	}
	if entity.EntityType != "CV" {
		t.Errorf("EntityType = %q, want CV", entity.EntityType)
	}
	if entity.NPWP != "99.888.777.6-543.000" {
		t.Errorf("NPWP = %q, want %q", entity.NPWP, "99.888.777.6-543.000")
	}
	if entity.Address != "Bandung, Jawa Barat" {
		t.Errorf("Address = %q, want %q", entity.Address, "Bandung, Jawa Barat")
	}
	if entity.CapitalAuth != 5000000000 {
		t.Errorf("CapitalAuth = %d, want 5000000000", entity.CapitalAuth)
	}
	if entity.CapitalPaid != 2500000000 {
		t.Errorf("CapitalPaid = %d, want 2500000000", entity.CapitalPaid)
	}
}

// TestRegister_ShareholderFetchError verifies that registration succeeds
// even when shareholder fetching fails (non-blocking).
func TestRegister_ShareholderFetchError(t *testing.T) {
	callerNIK := "3201010101010001"

	mock := &mockAHU{
		company: &ahu.CompanySearchResponse{
			Found:    true,
			SKNumber: "AHU-12345",
			Name:     "PT Test Corp",
		},
		officers: []ahu.Officer{
			{NIK: callerNIK, Name: "John Doe", Position: "DIREKTUR_UTAMA", AppointmentDate: "2020-01-01"},
		},
		shareholdersErr: fmt.Errorf("timeout fetching shareholders"),
	}

	entityStore := store.NewInMemoryEntityStore()
	roleStore := store.NewInMemoryRoleStore()

	h := NewRegisterHandler(RegisterDeps{
		AHU:         mock,
		EntityStore: entityStore,
		RoleStore:   roleStore,
		NIKKey:      testNIKKey,
	})

	body, _ := json.Marshal(map[string]string{
		"sk_number":      "AHU-12345",
		"caller_user_id": "user-1",
		"caller_nik":     callerNIK,
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/corp/register", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.Register(w, req)

	// Should still succeed even though shareholders failed
	if w.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
}

// TestRegister_ResponseFields verifies the response structure contains
// all expected fields.
func TestRegister_ResponseFields(t *testing.T) {
	callerNIK := "3201010101010001"

	mock := &mockAHU{
		company: &ahu.CompanySearchResponse{
			Found:    true,
			SKNumber: "AHU-12345",
			Name:     "PT Response Test",
		},
		officers: []ahu.Officer{
			{NIK: callerNIK, Name: "John Doe", Position: "DIREKTUR_UTAMA", AppointmentDate: "2020-01-01"},
		},
	}

	h := NewRegisterHandler(RegisterDeps{
		AHU:         mock,
		EntityStore: store.NewInMemoryEntityStore(),
		RoleStore:   store.NewInMemoryRoleStore(),
		NIKKey:      testNIKKey,
	})

	body, _ := json.Marshal(map[string]string{
		"sk_number":      "AHU-12345",
		"caller_user_id": "user-1",
		"caller_nik":     callerNIK,
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/corp/register", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.Register(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	// Verify response headers
	ct := w.Header().Get("Content-Type")
	if ct != "application/json; charset=utf-8" {
		t.Errorf("Content-Type = %q, want application/json; charset=utf-8", ct)
	}
	cc := w.Header().Get("Cache-Control")
	if cc != "no-store" {
		t.Errorf("Cache-Control = %q, want no-store", cc)
	}

	var resp registerResponse
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.EntityID == "" {
		t.Error("expected entity_id to be set")
	}
	if resp.Role != store.RoleRegisteredOfficer {
		t.Errorf("Role = %q, want %q", resp.Role, store.RoleRegisteredOfficer)
	}
}
