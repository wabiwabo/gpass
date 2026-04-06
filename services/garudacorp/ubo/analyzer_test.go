package ubo

import (
	"testing"
)

func TestAnalyze_SingleMajorityShareholder(t *testing.T) {
	a := NewAnalyzer()

	shareholders := []Shareholder{
		{Name: "Budi Santoso", NIKToken: "tok_budi", ShareType: "SAHAM_BIASA", Shares: 600, Percentage: 60.0},
		{Name: "Siti Aminah", NIKToken: "tok_siti", ShareType: "SAHAM_BIASA", Shares: 400, Percentage: 40.0},
	}

	result := a.Analyze("ent-1", "PT Maju Bersama", shareholders, nil)

	if result.Status != StatusIdentified {
		t.Fatalf("Status = %q, want %q", result.Status, StatusIdentified)
	}
	if len(result.BeneficialOwners) != 2 {
		t.Fatalf("expected 2 UBOs, got %d", len(result.BeneficialOwners))
	}
	// Both above 25%, sorted by percentage descending.
	if result.BeneficialOwners[0].Name != "Budi Santoso" {
		t.Errorf("first UBO = %q, want %q", result.BeneficialOwners[0].Name, "Budi Santoso")
	}
	if result.BeneficialOwners[0].Percentage != 60.0 {
		t.Errorf("Percentage = %f, want 60.0", result.BeneficialOwners[0].Percentage)
	}
	if result.BeneficialOwners[0].OwnershipType != OwnershipDirectShares {
		t.Errorf("OwnershipType = %q, want %q", result.BeneficialOwners[0].OwnershipType, OwnershipDirectShares)
	}
}

func TestAnalyze_SingleAboveThreshold(t *testing.T) {
	a := NewAnalyzer()

	shareholders := []Shareholder{
		{Name: "Budi Santoso", NIKToken: "tok_budi", ShareType: "SAHAM_BIASA", Shares: 600, Percentage: 60.0},
		{Name: "Siti Aminah", NIKToken: "tok_siti", ShareType: "SAHAM_BIASA", Shares: 200, Percentage: 20.0},
		{Name: "Rudi Hermawan", NIKToken: "tok_rudi", ShareType: "SAHAM_BIASA", Shares: 200, Percentage: 20.0},
	}

	result := a.Analyze("ent-1", "PT Maju Bersama", shareholders, nil)

	if result.Status != StatusIdentified {
		t.Fatalf("Status = %q, want %q", result.Status, StatusIdentified)
	}
	if len(result.BeneficialOwners) != 1 {
		t.Fatalf("expected 1 UBO, got %d", len(result.BeneficialOwners))
	}
	if result.BeneficialOwners[0].Name != "Budi Santoso" {
		t.Errorf("UBO = %q, want %q", result.BeneficialOwners[0].Name, "Budi Santoso")
	}
}

func TestAnalyze_MultipleAboveThreshold(t *testing.T) {
	a := NewAnalyzer()

	shareholders := []Shareholder{
		{Name: "Budi Santoso", NIKToken: "tok_budi", ShareType: "SAHAM_BIASA", Shares: 300, Percentage: 30.0},
		{Name: "Siti Aminah", NIKToken: "tok_siti", ShareType: "SAHAM_BIASA", Shares: 250, Percentage: 25.0},
		{Name: "Rudi Hermawan", NIKToken: "tok_rudi", ShareType: "SAHAM_BIASA", Shares: 200, Percentage: 20.0},
	}

	result := a.Analyze("ent-2", "PT Dua Pemilik", shareholders, nil)

	if result.Status != StatusIdentified {
		t.Fatalf("Status = %q, want %q", result.Status, StatusIdentified)
	}
	if len(result.BeneficialOwners) != 2 {
		t.Fatalf("expected 2 UBOs, got %d", len(result.BeneficialOwners))
	}
	// Sorted descending by percentage.
	if result.BeneficialOwners[0].Percentage != 30.0 {
		t.Errorf("first UBO percentage = %f, want 30.0", result.BeneficialOwners[0].Percentage)
	}
	if result.BeneficialOwners[1].Percentage != 25.0 {
		t.Errorf("second UBO percentage = %f, want 25.0", result.BeneficialOwners[1].Percentage)
	}
}

func TestAnalyze_NoneAboveThreshold_DirektUrControl(t *testing.T) {
	a := NewAnalyzer()

	shareholders := []Shareholder{
		{Name: "Budi Santoso", NIKToken: "tok_budi", ShareType: "SAHAM_BIASA", Shares: 200, Percentage: 20.0},
		{Name: "Siti Aminah", NIKToken: "tok_siti", ShareType: "SAHAM_BIASA", Shares: 150, Percentage: 15.0},
	}
	officers := []Officer{
		{Name: "Ahmad Wijaya", NIKToken: "tok_ahmad", Position: "DIREKTUR_UTAMA"},
		{Name: "Dewi Lestari", NIKToken: "tok_dewi", Position: "KOMISARIS"},
	}

	result := a.Analyze("ent-3", "PT Kontrol Direksi", shareholders, officers)

	if result.Status != StatusIdentified {
		t.Fatalf("Status = %q, want %q", result.Status, StatusIdentified)
	}
	if len(result.BeneficialOwners) != 1 {
		t.Fatalf("expected 1 UBO, got %d", len(result.BeneficialOwners))
	}
	ubo := result.BeneficialOwners[0]
	if ubo.Name != "Ahmad Wijaya" {
		t.Errorf("UBO = %q, want %q", ubo.Name, "Ahmad Wijaya")
	}
	if ubo.OwnershipType != OwnershipDirectorControl {
		t.Errorf("OwnershipType = %q, want %q", ubo.OwnershipType, OwnershipDirectorControl)
	}
	if ubo.Percentage != 0 {
		t.Errorf("Percentage = %f, want 0", ubo.Percentage)
	}
}

func TestAnalyze_InsufficientData_NoShareholdersNoOfficers(t *testing.T) {
	a := NewAnalyzer()

	result := a.Analyze("ent-4", "PT Kosong", nil, nil)

	if result.Status != StatusInsufficientData {
		t.Fatalf("Status = %q, want %q", result.Status, StatusInsufficientData)
	}
	if len(result.BeneficialOwners) != 0 {
		t.Errorf("expected 0 UBOs, got %d", len(result.BeneficialOwners))
	}
}

func TestAnalyze_InsufficientData_NoDirektUtama(t *testing.T) {
	a := NewAnalyzer()

	shareholders := []Shareholder{
		{Name: "Budi Santoso", NIKToken: "tok_budi", ShareType: "SAHAM_BIASA", Shares: 100, Percentage: 10.0},
	}
	officers := []Officer{
		{Name: "Dewi Lestari", NIKToken: "tok_dewi", Position: "KOMISARIS"},
	}

	result := a.Analyze("ent-5", "PT Tak Ada Direksi", shareholders, officers)

	if result.Status != StatusInsufficientData {
		t.Fatalf("Status = %q, want %q", result.Status, StatusInsufficientData)
	}
	if len(result.BeneficialOwners) != 0 {
		t.Errorf("expected 0 UBOs, got %d", len(result.BeneficialOwners))
	}
}

func TestAnalyze_ExactlyThreshold(t *testing.T) {
	a := NewAnalyzer()

	shareholders := []Shareholder{
		{Name: "Budi Santoso", NIKToken: "tok_budi", ShareType: "SAHAM_BIASA", Shares: 250, Percentage: 25.0},
		{Name: "Siti Aminah", NIKToken: "tok_siti", ShareType: "SAHAM_BIASA", Shares: 750, Percentage: 75.0},
	}

	result := a.Analyze("ent-6", "PT Pas Batas", shareholders, nil)

	if result.Status != StatusIdentified {
		t.Fatalf("Status = %q, want %q", result.Status, StatusIdentified)
	}
	if len(result.BeneficialOwners) != 2 {
		t.Fatalf("expected 2 UBOs, got %d", len(result.BeneficialOwners))
	}
}

func TestAnalyze_JustBelowThreshold(t *testing.T) {
	a := NewAnalyzer()

	shareholders := []Shareholder{
		{Name: "Budi Santoso", NIKToken: "tok_budi", ShareType: "SAHAM_BIASA", Shares: 2499, Percentage: 24.99},
	}

	result := a.Analyze("ent-7", "PT Bawah Batas", shareholders, nil)

	// 24.99% is below 25% threshold, and no officers means INSUFFICIENT_DATA.
	if result.Status != StatusInsufficientData {
		t.Fatalf("Status = %q, want %q", result.Status, StatusInsufficientData)
	}
	if len(result.BeneficialOwners) != 0 {
		t.Errorf("expected 0 UBOs, got %d", len(result.BeneficialOwners))
	}
}

func TestAnalyze_EmptyInput(t *testing.T) {
	a := NewAnalyzer()

	result := a.Analyze("ent-8", "PT Empty", []Shareholder{}, []Officer{})

	if result.Status != StatusInsufficientData {
		t.Fatalf("Status = %q, want %q", result.Status, StatusInsufficientData)
	}
	if len(result.BeneficialOwners) != 0 {
		t.Errorf("expected 0 UBOs, got %d", len(result.BeneficialOwners))
	}
}

func TestAnalyze_Metadata(t *testing.T) {
	a := NewAnalyzer()

	shareholders := []Shareholder{
		{Name: "Budi", NIKToken: "tok_budi", Percentage: 50.0},
	}

	result := a.Analyze("ent-9", "PT Meta", shareholders, nil)

	if result.EntityID != "ent-9" {
		t.Errorf("EntityID = %q, want %q", result.EntityID, "ent-9")
	}
	if result.EntityName != "PT Meta" {
		t.Errorf("EntityName = %q, want %q", result.EntityName, "PT Meta")
	}
	if result.Criteria != CriteriaPP132018 {
		t.Errorf("Criteria = %q, want %q", result.Criteria, CriteriaPP132018)
	}
	if result.AnalyzedAt == "" {
		t.Error("expected AnalyzedAt to be set")
	}
	if result.BeneficialOwners[0].Source != SourceAHU {
		t.Errorf("Source = %q, want %q", result.BeneficialOwners[0].Source, SourceAHU)
	}
	if result.BeneficialOwners[0].VerifiedAt == "" {
		t.Error("expected VerifiedAt to be set")
	}
}
