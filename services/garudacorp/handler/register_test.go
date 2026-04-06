package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/garudapass/gpass/services/garudacorp/ahu"
	"github.com/garudapass/gpass/services/garudacorp/store"
)

// mockAHU implements AHUVerifier for tests.
type mockAHU struct {
	company      *ahu.CompanySearchResponse
	companyErr   error
	officers     []ahu.Officer
	officersErr  error
	shareholders []ahu.Shareholder
	shareholdersErr error
}

func (m *mockAHU) SearchCompany(_ context.Context, _ string) (*ahu.CompanySearchResponse, error) {
	return m.company, m.companyErr
}

func (m *mockAHU) GetOfficers(_ context.Context, _ string) ([]ahu.Officer, error) {
	return m.officers, m.officersErr
}

func (m *mockAHU) GetShareholders(_ context.Context, _ string) ([]ahu.Shareholder, error) {
	return m.shareholders, m.shareholdersErr
}

var testNIKKey = []byte("01234567890123456789012345678901") // 32 bytes

func TestRegister_Success(t *testing.T) {
	callerNIK := "3201010101010001"

	mock := &mockAHU{
		company: &ahu.CompanySearchResponse{
			Found:       true,
			SKNumber:    "AHU-12345",
			Name:        "PT Test Corp",
			EntityType:  "PT",
			Status:      "ACTIVE",
			NPWP:        "01.234.567.8-901.000",
			Address:     "Jakarta",
			CapitalAuth: 1000000000,
			CapitalPaid: 500000000,
		},
		officers: []ahu.Officer{
			{NIK: callerNIK, Name: "John Doe", Position: "DIREKTUR_UTAMA", AppointmentDate: "2020-01-01"},
			{NIK: "3201010101010002", Name: "Jane Doe", Position: "KOMISARIS", AppointmentDate: "2020-01-01"},
		},
		shareholders: []ahu.Shareholder{
			{Name: "John Doe", ShareType: "INDIVIDUAL", Shares: 500, Percentage: 50.0},
		},
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

	if w.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d; body = %s", w.Code, http.StatusCreated, w.Body.String())
	}

	var resp registerResponse
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.EntityID == "" {
		t.Error("expected entity_id to be set")
	}
	if resp.Role != store.RoleRegisteredOfficer {
		t.Errorf("Role = %q, want %q", resp.Role, store.RoleRegisteredOfficer)
	}

	// Verify entity was created
	entity, err := entityStore.GetByID(context.Background(), resp.EntityID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if entity.Name != "PT Test Corp" {
		t.Errorf("entity Name = %q, want %q", entity.Name, "PT Test Corp")
	}
}

func TestRegister_CompanyNotFound(t *testing.T) {
	mock := &mockAHU{
		company: &ahu.CompanySearchResponse{Found: false},
	}

	h := NewRegisterHandler(RegisterDeps{
		AHU:         mock,
		EntityStore: store.NewInMemoryEntityStore(),
		RoleStore:   store.NewInMemoryRoleStore(),
		NIKKey:      testNIKKey,
	})

	body, _ := json.Marshal(map[string]string{
		"sk_number":      "AHU-99999",
		"caller_user_id": "user-1",
		"caller_nik":     "3201010101010001",
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/corp/register", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.Register(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", w.Code, http.StatusNotFound)
	}
}

func TestRegister_CallerNotOfficer(t *testing.T) {
	mock := &mockAHU{
		company: &ahu.CompanySearchResponse{
			Found:    true,
			SKNumber: "AHU-12345",
			Name:     "PT Test Corp",
		},
		officers: []ahu.Officer{
			{NIK: "3201010101010099", Name: "Someone Else", Position: "DIREKTUR_UTAMA", AppointmentDate: "2020-01-01"},
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
		"caller_nik":     "3201010101010001",
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/corp/register", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.Register(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("status = %d, want %d", w.Code, http.StatusForbidden)
	}
}

func TestRegister_CallerNotDirekturUtama(t *testing.T) {
	callerNIK := "3201010101010001"

	mock := &mockAHU{
		company: &ahu.CompanySearchResponse{
			Found:    true,
			SKNumber: "AHU-12345",
			Name:     "PT Test Corp",
		},
		officers: []ahu.Officer{
			{NIK: callerNIK, Name: "John Doe", Position: "KOMISARIS", AppointmentDate: "2020-01-01"},
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

	if w.Code != http.StatusForbidden {
		t.Errorf("status = %d, want %d", w.Code, http.StatusForbidden)
	}
}

func TestRegister_InvalidJSON(t *testing.T) {
	h := NewRegisterHandler(RegisterDeps{
		AHU:         &mockAHU{},
		EntityStore: store.NewInMemoryEntityStore(),
		RoleStore:   store.NewInMemoryRoleStore(),
		NIKKey:      testNIKKey,
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/corp/register", bytes.NewReader([]byte("not json")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.Register(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}
