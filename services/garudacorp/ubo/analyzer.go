package ubo

import (
	"sort"
	"time"
)

// Ownership type constants per PP 13/2018.
const (
	OwnershipDirectShares      = "DIRECT_SHARES"
	OwnershipControllingInt    = "CONTROLLING_INTEREST"
	OwnershipDirectorControl   = "DIRECTOR_CONTROL"
)

// Analysis status constants.
const (
	StatusIdentified       = "IDENTIFIED"
	StatusInsufficientData = "INSUFFICIENT_DATA"
	StatusPendingVerify    = "PENDING_VERIFICATION"
)

// Source constants.
const (
	SourceAHU    = "AHU"
	SourceManual = "MANUAL"
)

// CriteriaPP132018 is the regulatory criteria identifier.
const CriteriaPP132018 = "PP_13_2018"

// DefaultThreshold is the PP 13/2018 beneficial ownership threshold (25%).
const DefaultThreshold = 25.0

// BeneficialOwner represents an identified beneficial owner.
type BeneficialOwner struct {
	Name          string  `json:"name"`
	NIKToken      string  `json:"nik_token"`
	OwnershipType string  `json:"ownership_type"`
	Percentage    float64 `json:"percentage"`
	Source        string  `json:"source"`
	VerifiedAt    string  `json:"verified_at"`
}

// AnalysisResult contains the UBO analysis for an entity.
type AnalysisResult struct {
	EntityID         string            `json:"entity_id"`
	EntityName       string            `json:"entity_name"`
	BeneficialOwners []BeneficialOwner `json:"beneficial_owners"`
	AnalyzedAt       string            `json:"analyzed_at"`
	Criteria         string            `json:"criteria"`
	Status           string            `json:"status"`
}

// Shareholder is input data for analysis.
type Shareholder struct {
	Name       string
	NIKToken   string
	ShareType  string // SAHAM_BIASA, SAHAM_PREFEREN
	Shares     int64
	Percentage float64
}

// Officer is input data for analysis.
type Officer struct {
	Name     string
	NIKToken string
	Position string // DIREKTUR_UTAMA, DIREKTUR, KOMISARIS, etc.
}

// Analyzer identifies beneficial owners per PP 13/2018.
type Analyzer struct {
	threshold float64
}

// NewAnalyzer creates a UBO analyzer with the PP 13/2018 threshold (25%).
func NewAnalyzer() *Analyzer {
	return &Analyzer{
		threshold: DefaultThreshold,
	}
}

// Analyze determines beneficial owners from shareholders and officers.
func (a *Analyzer) Analyze(entityID, entityName string, shareholders []Shareholder, officers []Officer) *AnalysisResult {
	now := time.Now().UTC().Format(time.RFC3339)

	result := &AnalysisResult{
		EntityID:         entityID,
		EntityName:       entityName,
		BeneficialOwners: []BeneficialOwner{},
		AnalyzedAt:       now,
		Criteria:         CriteriaPP132018,
	}

	// Step 1: Check shareholders with >= threshold ownership.
	for _, sh := range shareholders {
		if sh.Percentage >= a.threshold {
			result.BeneficialOwners = append(result.BeneficialOwners, BeneficialOwner{
				Name:          sh.Name,
				NIKToken:      sh.NIKToken,
				OwnershipType: OwnershipDirectShares,
				Percentage:    sh.Percentage,
				Source:        SourceAHU,
				VerifiedAt:    now,
			})
		}
	}

	// Sort by percentage descending.
	sort.Slice(result.BeneficialOwners, func(i, j int) bool {
		return result.BeneficialOwners[i].Percentage > result.BeneficialOwners[j].Percentage
	})

	// Step 2: If no direct shareholders found, check for Direktur Utama.
	if len(result.BeneficialOwners) == 0 {
		for _, off := range officers {
			if off.Position == "DIREKTUR_UTAMA" {
				result.BeneficialOwners = append(result.BeneficialOwners, BeneficialOwner{
					Name:          off.Name,
					NIKToken:      off.NIKToken,
					OwnershipType: OwnershipDirectorControl,
					Percentage:    0,
					Source:        SourceAHU,
					VerifiedAt:    now,
				})
				break
			}
		}
	}

	// Set status based on results.
	if len(result.BeneficialOwners) > 0 {
		result.Status = StatusIdentified
	} else {
		result.Status = StatusInsufficientData
	}

	return result
}
