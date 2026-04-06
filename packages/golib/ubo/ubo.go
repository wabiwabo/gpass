// Package ubo implements Beneficial Ownership (UBO) analysis
// per PP 13/2018. Identifies natural persons who ultimately own
// or control a legal entity, using the 25% threshold for ownership
// and control chain traversal.
package ubo

import (
	"fmt"
	"sort"
)

// OwnershipThreshold is the PP 13/2018 threshold (25%).
const OwnershipThreshold = 25.0

// OwnerType classifies the owner.
type OwnerType string

const (
	TypeNaturalPerson OwnerType = "natural_person"
	TypeLegalEntity   OwnerType = "legal_entity"
	TypeGovernment    OwnerType = "government"
	TypeTrust         OwnerType = "trust"
)

// Owner represents an ownership node.
type Owner struct {
	ID              string    `json:"id"`
	Name            string    `json:"name"`
	Type            OwnerType `json:"type"`
	OwnershipPct    float64   `json:"ownership_pct"`
	VotingPct       float64   `json:"voting_pct,omitempty"`
	IsDirectControl bool      `json:"is_direct_control,omitempty"`
	Country         string    `json:"country,omitempty"`
	NIK             string    `json:"nik,omitempty"` // for natural persons
}

// IsBeneficialOwner checks if this owner meets the UBO threshold.
func (o Owner) IsBeneficialOwner() bool {
	return o.OwnershipPct >= OwnershipThreshold ||
		o.VotingPct >= OwnershipThreshold ||
		o.IsDirectControl
}

// OwnershipChain represents a chain of ownership from an entity
// to its beneficial owners.
type OwnershipChain struct {
	EntityID   string  `json:"entity_id"`
	EntityName string  `json:"entity_name"`
	Owners     []Owner `json:"owners"`
}

// AnalysisResult is the UBO analysis output.
type AnalysisResult struct {
	EntityID          string  `json:"entity_id"`
	BeneficialOwners  []Owner `json:"beneficial_owners"`
	TotalOwnership    float64 `json:"total_ownership_pct"`
	IsCompliant       bool    `json:"is_compliant"` // all UBOs identified
	UnidentifiedPct   float64 `json:"unidentified_pct,omitempty"`
	Warnings          []string `json:"warnings,omitempty"`
}

// Analyze performs UBO analysis on an ownership chain.
func Analyze(chain OwnershipChain) AnalysisResult {
	result := AnalysisResult{
		EntityID: chain.EntityID,
	}

	var totalPct float64
	for _, owner := range chain.Owners {
		totalPct += owner.OwnershipPct
		if owner.IsBeneficialOwner() {
			result.BeneficialOwners = append(result.BeneficialOwners, owner)
		}
	}

	result.TotalOwnership = totalPct

	// Sort by ownership percentage descending
	sort.Slice(result.BeneficialOwners, func(i, j int) bool {
		return result.BeneficialOwners[i].OwnershipPct > result.BeneficialOwners[j].OwnershipPct
	})

	// Check compliance
	var identifiedPct float64
	for _, bo := range result.BeneficialOwners {
		identifiedPct += bo.OwnershipPct
	}

	if totalPct > 100 {
		result.Warnings = append(result.Warnings, fmt.Sprintf("total ownership %.1f%% exceeds 100%%", totalPct))
	}

	// Unidentified = owners below threshold who are not natural persons
	for _, owner := range chain.Owners {
		if owner.Type == TypeLegalEntity && !owner.IsBeneficialOwner() {
			result.UnidentifiedPct += owner.OwnershipPct
			result.Warnings = append(result.Warnings,
				fmt.Sprintf("entity %q holds %.1f%% but UBO not identified through chain", owner.Name, owner.OwnershipPct))
		}
	}

	result.IsCompliant = result.UnidentifiedPct == 0 && len(result.BeneficialOwners) > 0

	return result
}

// EffectiveOwnership calculates effective ownership through a chain
// of intermediate entities.
// e.g., Person owns 60% of EntityA, EntityA owns 50% of EntityB
// → Person's effective ownership of EntityB = 60% * 50% = 30%
func EffectiveOwnership(percentages ...float64) float64 {
	if len(percentages) == 0 {
		return 0
	}
	result := percentages[0]
	for i := 1; i < len(percentages); i++ {
		result = (result * percentages[i]) / 100
	}
	return result
}

// MeetsThreshold checks if effective ownership meets the UBO threshold.
func MeetsThreshold(effectivePct float64) bool {
	return effectivePct >= OwnershipThreshold
}
