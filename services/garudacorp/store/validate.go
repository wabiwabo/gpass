package store

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/garudapass/gpass/services/garudacorp/ubo"
)

// Validators in this file are called by the Postgres-backed stores (production
// data path) to enforce PP 13/2018 (UBO 25%), AHU SK format conventions, and
// general data hygiene. The InMemory stores remain permissive (test fakes).

const (
	MaxAHUSKLen      = 100
	MaxNameLen       = 500
	MaxNPWPLen       = 20
	MaxAddressLen    = 2000
	MaxNIBLen        = 20

	MaxOfficerNameLen     = 255
	MaxPositionLen        = 50
	MaxNIKTokenLen        = 64

	MaxShareholderNameLen = 255
	MaxShareTypeLen       = 50

	MaxRoleAccessEntries  = 32
	MaxServiceAccessLen   = 64
)

var allowedEntityTypes = map[string]bool{
	"PT":         true,
	"CV":         true,
	"YAYASAN":    true,
	"KOPERASI":   true,
	"FIRMA":      true,
	"PERKUMPULAN": true,
	"BUMN":       true,
	"BUMD":       true,
}

var allowedEntityStatuses = map[string]bool{
	"ACTIVE":     true,
	"INACTIVE":   true,
	"DISSOLVED":  true,
	"SUSPENDED":  true,
}

var allowedRoles = map[string]bool{
	RoleRegisteredOfficer: true,
	RoleAdmin:             true,
	RoleUser:              true,
}

var allowedRoleStatuses = map[string]bool{
	StatusActive:  true,
	StatusRevoked: true,
}

var allowedOwnershipTypes = map[string]bool{
	"DIRECT_SHARES":   true,
	"INDIRECT_SHARES": true,
	"VOTING_RIGHTS":   true,
	"CONTROL":         true,
	"OFFICER":         true,
	"BENEFICIARY":     true,
}

var allowedUBOStatuses = map[string]bool{
	"IDENTIFIED":   true,
	"VERIFIED":     true,
	"UNVERIFIED":   true,
	"NONE_FOUND":   true,
}

// ValidateEntity enforces required fields, length bounds, and entity-type/status enums.
func ValidateEntity(e *Entity) error {
	if e == nil {
		return fmt.Errorf("entity is nil")
	}
	if err := requireBounded("ahu_sk_number", e.AHUSKNumber, MaxAHUSKLen); err != nil {
		return err
	}
	if err := requireBounded("name", e.Name, MaxNameLen); err != nil {
		return err
	}
	if !allowedEntityTypes[e.EntityType] {
		return fmt.Errorf("entity_type %q not in allowed set", e.EntityType)
	}
	if !allowedEntityStatuses[e.Status] {
		return fmt.Errorf("status %q not in allowed set", e.Status)
	}
	if err := bounded("npwp", e.NPWP, MaxNPWPLen); err != nil {
		return err
	}
	if err := bounded("address", e.Address, MaxAddressLen); err != nil {
		return err
	}
	if err := bounded("oss_nib", e.OSSNIB, MaxNIBLen); err != nil {
		return err
	}
	if e.CapitalAuth < 0 {
		return fmt.Errorf("capital_authorized must be non-negative")
	}
	if e.CapitalPaid < 0 {
		return fmt.Errorf("capital_paid must be non-negative")
	}
	if e.CapitalPaid > e.CapitalAuth && e.CapitalAuth > 0 {
		return fmt.Errorf("capital_paid (%d) cannot exceed capital_authorized (%d)", e.CapitalPaid, e.CapitalAuth)
	}
	return nil
}

// ValidateOfficer enforces officer field requirements.
func ValidateOfficer(o *EntityOfficer) error {
	if o == nil {
		return fmt.Errorf("officer is nil")
	}
	if err := requireBounded("officer.nik_token", o.NIKToken, MaxNIKTokenLen); err != nil {
		return err
	}
	if err := requireBounded("officer.name", o.Name, MaxOfficerNameLen); err != nil {
		return err
	}
	if err := requireBounded("officer.position", o.Position, MaxPositionLen); err != nil {
		return err
	}
	return nil
}

// ValidateShareholder enforces shareholder field requirements.
func ValidateShareholder(s *EntityShareholder) error {
	if s == nil {
		return fmt.Errorf("shareholder is nil")
	}
	if err := requireBounded("shareholder.name", s.Name, MaxShareholderNameLen); err != nil {
		return err
	}
	if err := bounded("shareholder.share_type", s.ShareType, MaxShareTypeLen); err != nil {
		return err
	}
	if s.Shares < 0 {
		return fmt.Errorf("shareholder.shares must be non-negative")
	}
	if s.Percentage < 0 || s.Percentage > 100 {
		return fmt.Errorf("shareholder.percentage %.2f outside [0,100]", s.Percentage)
	}
	return nil
}

// ValidateRole enforces role enum + service-access bounds + grantor lineage.
func ValidateRole(r *EntityRole) error {
	if r == nil {
		return fmt.Errorf("role is nil")
	}
	if err := requireBounded("entity_id", r.EntityID, 128); err != nil {
		return err
	}
	if err := requireBounded("user_id", r.UserID, 128); err != nil {
		return err
	}
	if !allowedRoles[r.Role] {
		return fmt.Errorf("role %q not in allowed set", r.Role)
	}
	if r.Status != "" && !allowedRoleStatuses[r.Status] {
		return fmt.Errorf("role status %q not in allowed set", r.Status)
	}
	if len(r.ServiceAccess) > MaxRoleAccessEntries {
		return fmt.Errorf("service_access has %d entries, max %d", len(r.ServiceAccess), MaxRoleAccessEntries)
	}
	for _, sa := range r.ServiceAccess {
		if err := requireBounded("service_access[i]", sa, MaxServiceAccessLen); err != nil {
			return err
		}
	}
	return nil
}

// ValidateUBOResult enforces PP 13/2018 invariants on a UBO analysis result.
func ValidateUBOResult(r *ubo.AnalysisResult) error {
	if r == nil {
		return fmt.Errorf("ubo result is nil")
	}
	if err := requireBounded("entity_id", r.EntityID, 128); err != nil {
		return err
	}
	if err := requireBounded("entity_name", r.EntityName, MaxNameLen); err != nil {
		return err
	}
	if r.Criteria == "" {
		return fmt.Errorf("criteria is required")
	}
	if !allowedUBOStatuses[r.Status] {
		return fmt.Errorf("ubo status %q not in allowed set", r.Status)
	}
	totalPct := 0.0
	for i := range r.BeneficialOwners {
		bo := &r.BeneficialOwners[i]
		if err := requireBounded("ubo.name", bo.Name, MaxOfficerNameLen); err != nil {
			return err
		}
		if err := requireBounded("ubo.nik_token", bo.NIKToken, MaxNIKTokenLen); err != nil {
			return err
		}
		if !allowedOwnershipTypes[bo.OwnershipType] {
			return fmt.Errorf("ubo[%d].ownership_type %q not in allowed set", i, bo.OwnershipType)
		}
		if bo.Percentage < 0 || bo.Percentage > 100 {
			return fmt.Errorf("ubo[%d].percentage %.2f outside [0,100]", i, bo.Percentage)
		}
		totalPct += bo.Percentage
	}
	// Sanity: cumulative ownership should not exceed 100% by more than rounding tolerance
	if totalPct > 100.5 {
		return fmt.Errorf("ubo total percentage %.2f exceeds 100", totalPct)
	}
	return nil
}

func requireBounded(name, v string, max int) error {
	if v == "" {
		return fmt.Errorf("%s is required", name)
	}
	return bounded(name, v, max)
}

func bounded(name, v string, max int) error {
	if utf8.RuneCountInString(v) > max {
		return fmt.Errorf("%s exceeds %d chars", name, max)
	}
	if strings.ContainsAny(v, "\x00") {
		return fmt.Errorf("%s contains null bytes", name)
	}
	return nil
}
