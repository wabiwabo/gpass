package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/garudapass/gpass/services/garudacorp/store"
	"github.com/garudapass/gpass/services/garudacorp/ubo"
)

func setupUBOTest(t *testing.T) (*UBOHandler, *store.InMemoryEntityStore, *store.InMemoryUBOStore) {
	t.Helper()
	entityStore := store.NewInMemoryEntityStore()
	uboStore := store.NewInMemoryUBOStore()
	handler := NewUBOHandler(entityStore, uboStore)
	return handler, entityStore, uboStore
}

func createTestEntity(t *testing.T, entityStore *store.InMemoryEntityStore) *store.Entity {
	t.Helper()
	ctx := context.Background()

	entity := &store.Entity{
		AHUSKNumber: "AHU-UBO-001",
		Name:        "PT UBO Test",
		EntityType:  "PT",
		Status:      "ACTIVE",
		NPWP:        "01.234.567.8-901.000",
	}
	if err := entityStore.Create(ctx, entity); err != nil {
		t.Fatalf("Create entity: %v", err)
	}

	entityStore.AddShareholders(ctx, entity.ID, []store.EntityShareholder{
		{Name: "Budi Santoso", ShareType: "SAHAM_BIASA", Shares: 600, Percentage: 60.0},
		{Name: "Siti Aminah", ShareType: "SAHAM_BIASA", Shares: 400, Percentage: 40.0},
	})
	entityStore.AddOfficers(ctx, entity.ID, []store.EntityOfficer{
		{NIKToken: "tok_budi", Name: "Budi Santoso", Position: "DIREKTUR_UTAMA"},
	})

	return entity
}

func TestAnalyzeUBO_Success(t *testing.T) {
	h, entityStore, _ := setupUBOTest(t)
	entity := createTestEntity(t, entityStore)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/corp/entities/"+entity.ID+"/ubo/analyze", nil)
	req.SetPathValue("entity_id", entity.ID)
	w := httptest.NewRecorder()

	h.AnalyzeUBO(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body = %s", w.Code, http.StatusOK, w.Body.String())
	}

	var result ubo.AnalysisResult
	if err := json.NewDecoder(w.Body).Decode(&result); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if result.Status != ubo.StatusIdentified {
		t.Errorf("Status = %q, want %q", result.Status, ubo.StatusIdentified)
	}
	if len(result.BeneficialOwners) != 2 {
		t.Fatalf("expected 2 UBOs, got %d", len(result.BeneficialOwners))
	}
	// Both 60% and 40% are above 25%.
	if result.BeneficialOwners[0].Percentage != 60.0 {
		t.Errorf("first UBO percentage = %f, want 60.0", result.BeneficialOwners[0].Percentage)
	}
	if result.BeneficialOwners[0].OwnershipType != ubo.OwnershipDirectShares {
		t.Errorf("OwnershipType = %q, want %q", result.BeneficialOwners[0].OwnershipType, ubo.OwnershipDirectShares)
	}
	if result.Criteria != ubo.CriteriaPP132018 {
		t.Errorf("Criteria = %q, want %q", result.Criteria, ubo.CriteriaPP132018)
	}
}

func TestAnalyzeUBO_EntityNotFound(t *testing.T) {
	h, _, _ := setupUBOTest(t)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/corp/entities/nonexistent/ubo/analyze", nil)
	req.SetPathValue("entity_id", "nonexistent")
	w := httptest.NewRecorder()

	h.AnalyzeUBO(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", w.Code, http.StatusNotFound)
	}
}

func TestGetUBO_Success(t *testing.T) {
	h, entityStore, uboStore := setupUBOTest(t)
	entity := createTestEntity(t, entityStore)

	// Pre-save a result.
	uboStore.Save(&ubo.AnalysisResult{
		EntityID:   entity.ID,
		EntityName: entity.Name,
		BeneficialOwners: []ubo.BeneficialOwner{
			{Name: "Budi Santoso", OwnershipType: ubo.OwnershipDirectShares, Percentage: 60.0},
		},
		Criteria: ubo.CriteriaPP132018,
		Status:   ubo.StatusIdentified,
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/corp/entities/"+entity.ID+"/ubo", nil)
	req.SetPathValue("entity_id", entity.ID)
	w := httptest.NewRecorder()

	h.GetUBO(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body = %s", w.Code, http.StatusOK, w.Body.String())
	}

	var result ubo.AnalysisResult
	if err := json.NewDecoder(w.Body).Decode(&result); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if result.EntityID != entity.ID {
		t.Errorf("EntityID = %q, want %q", result.EntityID, entity.ID)
	}
	if len(result.BeneficialOwners) != 1 {
		t.Fatalf("expected 1 UBO, got %d", len(result.BeneficialOwners))
	}
}

func TestGetUBO_NotFound(t *testing.T) {
	h, _, _ := setupUBOTest(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/corp/entities/nonexistent/ubo", nil)
	req.SetPathValue("entity_id", "nonexistent")
	w := httptest.NewRecorder()

	h.GetUBO(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", w.Code, http.StatusNotFound)
	}
}
