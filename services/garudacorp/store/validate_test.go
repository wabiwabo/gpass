package store

import (
	"strings"
	"testing"
	"time"

	"github.com/garudapass/gpass/services/garudacorp/ubo"
)

func validEntity() *Entity {
	return &Entity{
		AHUSKNumber:   "AHU-001",
		Name:          "PT Test",
		EntityType:    "PT",
		Status:        "ACTIVE",
		NPWP:          "01.234.567.8-901.000",
		CapitalAuth:   1000000000,
		CapitalPaid:   500000000,
		AHUVerifiedAt: time.Now(),
	}
}

func TestValidateEntity_Valid(t *testing.T) {
	if err := ValidateEntity(validEntity()); err != nil {
		t.Errorf("expected nil, got %v", err)
	}
}

func TestValidateEntity_Required(t *testing.T) {
	cases := []func(*Entity){
		func(e *Entity) { e.AHUSKNumber = "" },
		func(e *Entity) { e.Name = "" },
		func(e *Entity) { e.EntityType = "FOO" },
		func(e *Entity) { e.Status = "PENDING" },
	}
	for i, mut := range cases {
		e := validEntity()
		mut(e)
		if err := ValidateEntity(e); err == nil {
			t.Errorf("case %d: expected error", i)
		}
	}
}

func TestValidateEntity_CapitalInvariants(t *testing.T) {
	e := validEntity()
	e.CapitalAuth = -1
	if err := ValidateEntity(e); err == nil {
		t.Error("expected negative capital error")
	}
	e = validEntity()
	e.CapitalAuth = 100
	e.CapitalPaid = 200
	if err := ValidateEntity(e); err == nil {
		t.Error("expected paid > auth error")
	}
}

func TestValidateEntity_Nil(t *testing.T) {
	if err := ValidateEntity(nil); err == nil {
		t.Error("expected nil rejection")
	}
}

func TestValidateOfficer(t *testing.T) {
	o := &EntityOfficer{NIKToken: "tok", Name: "Budi", Position: "DIREKTUR"}
	if err := ValidateOfficer(o); err != nil {
		t.Errorf("valid: %v", err)
	}
	if err := ValidateOfficer(&EntityOfficer{Name: "x", Position: "y"}); err == nil {
		t.Error("expected nik_token required")
	}
}

func TestValidateShareholder(t *testing.T) {
	s := &EntityShareholder{Name: "Budi", Shares: 100, Percentage: 50.0}
	if err := ValidateShareholder(s); err != nil {
		t.Errorf("valid: %v", err)
	}
	bad := &EntityShareholder{Name: "Budi", Percentage: 150.0}
	if err := ValidateShareholder(bad); err == nil {
		t.Error("expected percentage out of range")
	}
	bad2 := &EntityShareholder{Name: "Budi", Shares: -1}
	if err := ValidateShareholder(bad2); err == nil {
		t.Error("expected negative shares")
	}
}

func TestValidateRole_Valid(t *testing.T) {
	r := &EntityRole{
		EntityID:      "ent-1",
		UserID:        "user-1",
		Role:          RoleAdmin,
		Status:        StatusActive,
		ServiceAccess: []string{"signing", "garudainfo"},
	}
	if err := ValidateRole(r); err != nil {
		t.Errorf("valid: %v", err)
	}
}

func TestValidateRole_Bad(t *testing.T) {
	r := &EntityRole{EntityID: "e", UserID: "u", Role: "GOD"}
	if err := ValidateRole(r); err == nil {
		t.Error("expected role enum violation")
	}

	r = &EntityRole{
		EntityID:      "e",
		UserID:        "u",
		Role:          RoleUser,
		ServiceAccess: make([]string, MaxRoleAccessEntries+1),
	}
	for i := range r.ServiceAccess {
		r.ServiceAccess[i] = "svc"
	}
	if err := ValidateRole(r); err == nil {
		t.Error("expected too-many entries")
	}
}

func TestValidateUBOResult_Valid(t *testing.T) {
	r := &ubo.AnalysisResult{
		EntityID:   "ent-1",
		EntityName: "PT Test",
		Criteria:   "PP_13_2018",
		Status:     "IDENTIFIED",
		BeneficialOwners: []ubo.BeneficialOwner{
			{Name: "Budi", NIKToken: "tok1", OwnershipType: "DIRECT_SHARES", Percentage: 60.0},
			{Name: "Siti", NIKToken: "tok2", OwnershipType: "DIRECT_SHARES", Percentage: 30.0},
		},
	}
	if err := ValidateUBOResult(r); err != nil {
		t.Errorf("valid: %v", err)
	}
}

func TestValidateUBOResult_TotalExceeds100(t *testing.T) {
	r := &ubo.AnalysisResult{
		EntityID: "e", EntityName: "n", Criteria: "PP_13_2018", Status: "IDENTIFIED",
		BeneficialOwners: []ubo.BeneficialOwner{
			{Name: "A", NIKToken: "t", OwnershipType: "DIRECT_SHARES", Percentage: 60},
			{Name: "B", NIKToken: "t", OwnershipType: "DIRECT_SHARES", Percentage: 60},
		},
	}
	if err := ValidateUBOResult(r); err == nil {
		t.Error("expected total > 100")
	}
}

func TestValidateUBOResult_BadOwnershipType(t *testing.T) {
	r := &ubo.AnalysisResult{
		EntityID: "e", EntityName: "n", Criteria: "PP_13_2018", Status: "IDENTIFIED",
		BeneficialOwners: []ubo.BeneficialOwner{
			{Name: "A", NIKToken: "t", OwnershipType: "MAGIC", Percentage: 30},
		},
	}
	if err := ValidateUBOResult(r); err == nil {
		t.Error("expected ownership_type rejection")
	}
}

func TestValidate_NullByte(t *testing.T) {
	e := validEntity()
	e.Name = "evil\x00pt"
	err := ValidateEntity(e)
	if err == nil || !strings.Contains(err.Error(), "null") {
		t.Errorf("expected null byte error, got %v", err)
	}
}
