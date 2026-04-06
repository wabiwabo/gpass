package ubo

import (
	"testing"
)

func TestOwnershipThreshold(t *testing.T) {
	if OwnershipThreshold != 25.0 {
		t.Errorf("OwnershipThreshold = %f, want 25.0", OwnershipThreshold)
	}
}

func TestOwner_IsBeneficialOwner(t *testing.T) {
	tests := []struct {
		name  string
		owner Owner
		want  bool
	}{
		{"above threshold", Owner{OwnershipPct: 30}, true},
		{"at threshold", Owner{OwnershipPct: 25}, true},
		{"below threshold", Owner{OwnershipPct: 20}, false},
		{"voting above threshold", Owner{VotingPct: 30}, true},
		{"voting at threshold", Owner{VotingPct: 25}, true},
		{"direct control", Owner{OwnershipPct: 10, IsDirectControl: true}, true},
		{"zero ownership", Owner{OwnershipPct: 0}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.owner.IsBeneficialOwner(); got != tt.want {
				t.Errorf("IsBeneficialOwner = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAnalyze_SimpleMajority(t *testing.T) {
	chain := OwnershipChain{
		EntityID:   "ent-1",
		EntityName: "PT Contoh",
		Owners: []Owner{
			{ID: "person-1", Name: "Budi", Type: TypeNaturalPerson, OwnershipPct: 60},
			{ID: "person-2", Name: "Siti", Type: TypeNaturalPerson, OwnershipPct: 40},
		},
	}

	result := Analyze(chain)

	if !result.IsCompliant {
		t.Error("should be compliant")
	}
	if len(result.BeneficialOwners) != 2 {
		t.Errorf("BeneficialOwners = %d, want 2", len(result.BeneficialOwners))
	}
	if result.TotalOwnership != 100 {
		t.Errorf("TotalOwnership = %f", result.TotalOwnership)
	}
	// Sorted by ownership descending
	if result.BeneficialOwners[0].Name != "Budi" {
		t.Errorf("first UBO = %q, want Budi (60%%)", result.BeneficialOwners[0].Name)
	}
}

func TestAnalyze_BelowThreshold(t *testing.T) {
	chain := OwnershipChain{
		EntityID: "ent-1",
		Owners: []Owner{
			{ID: "p1", Type: TypeNaturalPerson, OwnershipPct: 10},
			{ID: "p2", Type: TypeNaturalPerson, OwnershipPct: 10},
			{ID: "p3", Type: TypeNaturalPerson, OwnershipPct: 10},
			{ID: "p4", Type: TypeNaturalPerson, OwnershipPct: 10},
			{ID: "p5", Type: TypeNaturalPerson, OwnershipPct: 10},
		},
	}

	result := Analyze(chain)
	if len(result.BeneficialOwners) != 0 {
		t.Errorf("BeneficialOwners = %d, want 0 (all below 25%%)", len(result.BeneficialOwners))
	}
	if result.IsCompliant {
		t.Error("should not be compliant (no UBOs identified)")
	}
}

func TestAnalyze_DirectControl(t *testing.T) {
	chain := OwnershipChain{
		EntityID: "ent-1",
		Owners: []Owner{
			{ID: "p1", Name: "Director", Type: TypeNaturalPerson, OwnershipPct: 10, IsDirectControl: true},
			{ID: "p2", Name: "Minor", Type: TypeNaturalPerson, OwnershipPct: 90},
		},
	}

	result := Analyze(chain)
	if len(result.BeneficialOwners) != 2 {
		t.Errorf("BeneficialOwners = %d, want 2", len(result.BeneficialOwners))
	}
}

func TestAnalyze_UnidentifiedEntity(t *testing.T) {
	chain := OwnershipChain{
		EntityID: "ent-1",
		Owners: []Owner{
			{ID: "p1", Type: TypeNaturalPerson, OwnershipPct: 50},
			{ID: "e1", Name: "PT Shadow", Type: TypeLegalEntity, OwnershipPct: 20}, // below threshold, UBO unknown
		},
	}

	result := Analyze(chain)
	if result.IsCompliant {
		t.Error("should not be compliant (entity without identified UBO)")
	}
	if result.UnidentifiedPct != 20 {
		t.Errorf("UnidentifiedPct = %f, want 20", result.UnidentifiedPct)
	}
	if len(result.Warnings) == 0 {
		t.Error("should have warnings")
	}
}

func TestAnalyze_OverHundredPercent(t *testing.T) {
	chain := OwnershipChain{
		EntityID: "ent-1",
		Owners: []Owner{
			{ID: "p1", Type: TypeNaturalPerson, OwnershipPct: 60},
			{ID: "p2", Type: TypeNaturalPerson, OwnershipPct: 60},
		},
	}

	result := Analyze(chain)
	if result.TotalOwnership != 120 {
		t.Errorf("TotalOwnership = %f", result.TotalOwnership)
	}

	hasWarning := false
	for _, w := range result.Warnings {
		if len(w) > 0 {
			hasWarning = true
		}
	}
	if !hasWarning {
		t.Error("should warn about >100% ownership")
	}
}

func TestAnalyze_EmptyOwners(t *testing.T) {
	chain := OwnershipChain{EntityID: "ent-1"}
	result := Analyze(chain)
	if result.IsCompliant {
		t.Error("no owners = not compliant")
	}
	if len(result.BeneficialOwners) != 0 {
		t.Error("should have no beneficial owners")
	}
}

func TestEffectiveOwnership(t *testing.T) {
	tests := []struct {
		name  string
		pcts  []float64
		want  float64
	}{
		{"direct 60%", []float64{60}, 60},
		{"two levels 60% * 50%", []float64{60, 50}, 30},
		{"three levels 80% * 50% * 50%", []float64{80, 50, 50}, 20},
		{"100% * 100%", []float64{100, 100}, 100},
		{"empty", nil, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := EffectiveOwnership(tt.pcts...)
			if got != tt.want {
				t.Errorf("EffectiveOwnership(%v) = %f, want %f", tt.pcts, got, tt.want)
			}
		})
	}
}

func TestMeetsThreshold(t *testing.T) {
	tests := []struct {
		pct  float64
		want bool
	}{
		{30, true},
		{25, true},
		{24.9, false},
		{0, false},
		{100, true},
	}

	for _, tt := range tests {
		if got := MeetsThreshold(tt.pct); got != tt.want {
			t.Errorf("MeetsThreshold(%f) = %v, want %v", tt.pct, got, tt.want)
		}
	}
}

func TestEffectiveOwnership_RealWorld(t *testing.T) {
	// Person owns 60% of Entity A, which owns 50% of Entity B
	// Effective ownership of Entity B = 30% (above threshold)
	effective := EffectiveOwnership(60, 50)
	if !MeetsThreshold(effective) {
		t.Errorf("60%% * 50%% = %.1f%%, should meet threshold", effective)
	}

	// Person owns 40% of Entity A, which owns 50% of Entity B
	// Effective = 20% (below threshold)
	effective2 := EffectiveOwnership(40, 50)
	if MeetsThreshold(effective2) {
		t.Errorf("40%% * 50%% = %.1f%%, should not meet threshold", effective2)
	}
}

func TestOwnerTypes(t *testing.T) {
	types := []OwnerType{TypeNaturalPerson, TypeLegalEntity, TypeGovernment, TypeTrust}
	seen := make(map[OwnerType]bool)
	for _, typ := range types {
		if typ == "" {
			t.Error("type should not be empty")
		}
		if seen[typ] {
			t.Errorf("duplicate type: %q", typ)
		}
		seen[typ] = true
	}
}

func TestAnalyze_VotingThreshold(t *testing.T) {
	chain := OwnershipChain{
		EntityID: "ent-1",
		Owners: []Owner{
			{ID: "p1", Type: TypeNaturalPerson, OwnershipPct: 10, VotingPct: 30},
			{ID: "p2", Type: TypeNaturalPerson, OwnershipPct: 40},
		},
	}

	result := Analyze(chain)
	if len(result.BeneficialOwners) != 2 {
		t.Errorf("BeneficialOwners = %d, want 2 (p1 by voting, p2 by ownership)", len(result.BeneficialOwners))
	}
}
