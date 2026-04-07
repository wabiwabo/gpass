package store

import (
	"testing"
)

// TestValidateEntity_Nil_Cov pins nil guard.
func TestValidateEntity_Nil_Cov(t *testing.T) {
	if err := ValidateEntity(nil); err == nil {
		t.Error("nil accepted")
	}
}

// TestValidateEntity_CapitalNegative pins the negative-capital branches.
func TestValidateEntity_CapitalInvariants_Cov(t *testing.T) {
	base := func() *Entity {
		return &Entity{
			AHUSKNumber: "AHU-001", Name: "PT X", EntityType: "PT", Status: "ACTIVE",
		}
	}
	e := base()
	e.CapitalAuth = -1
	if err := ValidateEntity(e); err == nil {
		t.Error("negative auth accepted")
	}
	e = base()
	e.CapitalPaid = -1
	if err := ValidateEntity(e); err == nil {
		t.Error("negative paid accepted")
	}
	e = base()
	e.CapitalAuth = 100
	e.CapitalPaid = 200
	if err := ValidateEntity(e); err == nil {
		t.Error("paid > auth accepted")
	}
}

// TestValidateEntity_BadType pins the allowed-entity-type enum.
func TestValidateEntity_BadType(t *testing.T) {
	e := &Entity{
		AHUSKNumber: "AHU-001", Name: "X", EntityType: "BOGUS", Status: "ACTIVE",
	}
	if err := ValidateEntity(e); err == nil {
		t.Error("bad type accepted")
	}
}

// TestValidateEntity_BadStatus pins the allowed-status enum.
func TestValidateEntity_BadStatus(t *testing.T) {
	e := &Entity{
		AHUSKNumber: "AHU-001", Name: "X", EntityType: "PT", Status: "BOGUS",
	}
	if err := ValidateEntity(e); err == nil {
		t.Error("bad status accepted")
	}
}

// TestValidateOfficer_Nil pins nil guard.
func TestValidateOfficer_Nil(t *testing.T) {
	if err := ValidateOfficer(nil); err == nil {
		t.Error("nil accepted")
	}
}

// TestValidateShareholder_Nil pins nil guard.
func TestValidateShareholder_Nil(t *testing.T) {
	if err := ValidateShareholder(nil); err == nil {
		t.Error("nil accepted")
	}
}

// TestValidateShareholder_BadPercentage pins out-of-range percentage.
func TestValidateShareholder_BadPercentage(t *testing.T) {
	s := &EntityShareholder{Name: "X", Percentage: 150}
	if err := ValidateShareholder(s); err == nil {
		t.Error("percentage > 100 accepted")
	}
	s = &EntityShareholder{Name: "X", Percentage: -1}
	if err := ValidateShareholder(s); err == nil {
		t.Error("percentage < 0 accepted")
	}
	s = &EntityShareholder{Name: "X", Shares: -1}
	if err := ValidateShareholder(s); err == nil {
		t.Error("negative shares accepted")
	}
}
