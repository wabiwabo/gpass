package store

import (
	"testing"

	"github.com/garudapass/gpass/services/garudacorp/ubo"
)

func TestUBOStore_SaveAndGet(t *testing.T) {
	s := NewInMemoryUBOStore()

	result := &ubo.AnalysisResult{
		EntityID:   "ent-1",
		EntityName: "PT Test Corp",
		BeneficialOwners: []ubo.BeneficialOwner{
			{Name: "Budi", NIKToken: "tok_budi", OwnershipType: "DIRECT_SHARES", Percentage: 60.0, Source: "AHU"},
		},
		AnalyzedAt: "2026-04-06T10:00:00Z",
		Criteria:   "PP_13_2018",
		Status:     "IDENTIFIED",
	}

	if err := s.Save(result); err != nil {
		t.Fatalf("Save: %v", err)
	}

	got, err := s.GetByEntityID("ent-1")
	if err != nil {
		t.Fatalf("GetByEntityID: %v", err)
	}
	if got.EntityName != "PT Test Corp" {
		t.Errorf("EntityName = %q, want %q", got.EntityName, "PT Test Corp")
	}
	if len(got.BeneficialOwners) != 1 {
		t.Fatalf("expected 1 UBO, got %d", len(got.BeneficialOwners))
	}
	if got.BeneficialOwners[0].Name != "Budi" {
		t.Errorf("UBO Name = %q, want %q", got.BeneficialOwners[0].Name, "Budi")
	}
}

func TestUBOStore_Update(t *testing.T) {
	s := NewInMemoryUBOStore()

	first := &ubo.AnalysisResult{
		EntityID:   "ent-1",
		EntityName: "PT Test Corp",
		BeneficialOwners: []ubo.BeneficialOwner{
			{Name: "Budi", Percentage: 60.0},
		},
		Status: "IDENTIFIED",
	}
	s.Save(first)

	second := &ubo.AnalysisResult{
		EntityID:   "ent-1",
		EntityName: "PT Test Corp",
		BeneficialOwners: []ubo.BeneficialOwner{
			{Name: "Budi", Percentage: 40.0},
			{Name: "Siti", Percentage: 30.0},
		},
		Status: "IDENTIFIED",
	}
	s.Save(second)

	got, err := s.GetByEntityID("ent-1")
	if err != nil {
		t.Fatalf("GetByEntityID: %v", err)
	}
	if len(got.BeneficialOwners) != 2 {
		t.Fatalf("expected 2 UBOs after update, got %d", len(got.BeneficialOwners))
	}
}

func TestUBOStore_NotFound(t *testing.T) {
	s := NewInMemoryUBOStore()

	_, err := s.GetByEntityID("nonexistent")
	if err != ErrUBONotFound {
		t.Errorf("expected ErrUBONotFound, got %v", err)
	}
}

func TestUBOStore_ListAll(t *testing.T) {
	s := NewInMemoryUBOStore()

	s.Save(&ubo.AnalysisResult{EntityID: "ent-1", EntityName: "PT A", Status: "IDENTIFIED"})
	s.Save(&ubo.AnalysisResult{EntityID: "ent-2", EntityName: "PT B", Status: "INSUFFICIENT_DATA"})

	all, err := s.ListAll()
	if err != nil {
		t.Fatalf("ListAll: %v", err)
	}
	if len(all) != 2 {
		t.Fatalf("expected 2 results, got %d", len(all))
	}
}

func TestUBOStore_ListAll_Empty(t *testing.T) {
	s := NewInMemoryUBOStore()

	all, err := s.ListAll()
	if err != nil {
		t.Fatalf("ListAll: %v", err)
	}
	if len(all) != 0 {
		t.Errorf("expected 0 results, got %d", len(all))
	}
}

func TestUBOStore_CopyIsolation(t *testing.T) {
	s := NewInMemoryUBOStore()

	result := &ubo.AnalysisResult{
		EntityID: "ent-1",
		BeneficialOwners: []ubo.BeneficialOwner{
			{Name: "Budi"},
		},
	}
	s.Save(result)

	// Mutate the original.
	result.BeneficialOwners[0].Name = "Modified"

	got, _ := s.GetByEntityID("ent-1")
	if got.BeneficialOwners[0].Name != "Budi" {
		t.Errorf("store was mutated externally: got %q, want %q", got.BeneficialOwners[0].Name, "Budi")
	}
}
