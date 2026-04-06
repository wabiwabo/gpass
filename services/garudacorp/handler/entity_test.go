package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/garudapass/gpass/services/garudacorp/oss"
	"github.com/garudapass/gpass/services/garudacorp/store"
)

// mockOSS implements OSSSearcher for tests.
type mockOSS struct {
	resp *oss.NIBSearchResponse
	err  error
}

func (m *mockOSS) SearchByNPWP(_ context.Context, _ string) (*oss.NIBSearchResponse, error) {
	return m.resp, m.err
}

func (m *mockOSS) SearchByNIB(_ context.Context, _ string) (*oss.NIBSearchResponse, error) {
	return m.resp, m.err
}

func TestGetEntity_Success(t *testing.T) {
	entityStore := store.NewInMemoryEntityStore()
	ctx := context.Background()

	entity := &store.Entity{
		AHUSKNumber: "AHU-12345",
		Name:        "PT Test Corp",
		EntityType:  "PT",
		Status:      "ACTIVE",
		NPWP:        "01.234.567.8-901.000",
		Address:     "Jakarta",
		CapitalAuth: 1000000000,
		CapitalPaid: 500000000,
	}
	entityStore.Create(ctx, entity)
	entityStore.AddOfficers(ctx, entity.ID, []store.EntityOfficer{
		{NIKToken: "token1", Name: "John Doe", Position: "DIREKTUR_UTAMA"},
	})

	ossClient := &mockOSS{
		resp: &oss.NIBSearchResponse{
			Found:  true,
			NIB:    "1234567890123",
			Name:   "PT Test Corp",
			Status: "ACTIVE",
			Businesses: []oss.Business{
				{KBLI: "62011", Description: "IT Consulting", Status: "ACTIVE", RiskLevel: "LOW"},
			},
		},
	}

	h := NewEntityHandler(entityStore, ossClient)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/corp/entities/"+entity.ID, nil)
	req.SetPathValue("id", entity.ID)
	w := httptest.NewRecorder()

	h.GetEntity(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body = %s", w.Code, http.StatusOK, w.Body.String())
	}

	var resp entityResponse
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.Name != "PT Test Corp" {
		t.Errorf("Name = %q, want %q", resp.Name, "PT Test Corp")
	}
	if resp.OSSNIB != "1234567890123" {
		t.Errorf("OSSNIB = %q, want %q", resp.OSSNIB, "1234567890123")
	}
	if len(resp.OSSBusinesses) != 1 {
		t.Errorf("expected 1 OSS business, got %d", len(resp.OSSBusinesses))
	}
	if len(resp.Officers) != 1 {
		t.Errorf("expected 1 officer, got %d", len(resp.Officers))
	}
}

func TestGetEntity_NotFound(t *testing.T) {
	entityStore := store.NewInMemoryEntityStore()
	h := NewEntityHandler(entityStore, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/corp/entities/nonexistent", nil)
	req.SetPathValue("id", "nonexistent")
	w := httptest.NewRecorder()

	h.GetEntity(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", w.Code, http.StatusNotFound)
	}
}

func TestGetEntity_OSSFailure_StillReturnsEntity(t *testing.T) {
	entityStore := store.NewInMemoryEntityStore()
	ctx := context.Background()

	entity := &store.Entity{
		AHUSKNumber: "AHU-12345",
		Name:        "PT Test Corp",
		EntityType:  "PT",
		Status:      "ACTIVE",
		NPWP:        "01.234.567.8-901.000",
	}
	entityStore.Create(ctx, entity)

	ossClient := &mockOSS{
		err: fmt.Errorf("OSS service unavailable"),
	}

	h := NewEntityHandler(entityStore, ossClient)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/corp/entities/"+entity.ID, nil)
	req.SetPathValue("id", entity.ID)
	w := httptest.NewRecorder()

	h.GetEntity(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var resp entityResponse
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.Name != "PT Test Corp" {
		t.Errorf("Name = %q, want %q", resp.Name, "PT Test Corp")
	}
	if resp.OSSNIB != "" {
		t.Errorf("OSSNIB = %q, want empty (OSS failed)", resp.OSSNIB)
	}
}
