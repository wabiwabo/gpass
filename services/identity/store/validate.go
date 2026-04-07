package store

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

// Validation limits for deletion requests (UU PDP No. 27/2022 right-to-deletion).
const (
	MaxUserIDLen      = 128
	MaxStatusLen      = 32
	MaxDataCategories = 32
	MaxCategoryLen    = 64
)

var allowedStatuses = map[string]bool{
	"":           true, // empty defaults to PENDING
	"PENDING":    true,
	"PROCESSING": true,
	"COMPLETED":  true,
	"FAILED":     true,
}

// allowedDataCategories is the closed set of personal-data categories that can
// be marked deleted. New categories require an explicit code change + DPO sign-off.
var allowedDataCategories = map[string]bool{
	"personal_info":    true,
	"biometric":        true,
	"contact":          true,
	"identity":         true,
	"financial":        true,
	"health":           true,
	"location":         true,
	"family":           true,
	"document":         true,
	"signature":        true,
	"audit_trail":      true,
	"consent_history":  true,
	"behavioral":       true,
	"device":           true,
	"session":          true,
}

// ValidateDeletionRequest enforces required fields, length limits, and the
// reason allow-list. Called by both InMemory and Postgres impls.
func ValidateDeletionRequest(r *DeletionRequest) error {
	if r == nil {
		return fmt.Errorf("deletion request is nil")
	}
	if r.UserID == "" {
		return fmt.Errorf("user_id is required")
	}
	if utf8.RuneCountInString(r.UserID) > MaxUserIDLen {
		return fmt.Errorf("user_id exceeds %d chars", MaxUserIDLen)
	}
	if strings.ContainsAny(r.UserID, "\x00\n\r") {
		return fmt.Errorf("user_id contains control characters")
	}
	if !ValidReasons[r.Reason] {
		return ErrInvalidReason
	}
	return nil
}

// ValidateStatusUpdate enforces enum and category allow-lists for UpdateStatus.
func ValidateStatusUpdate(status string, deletedData []string) error {
	if !allowedStatuses[status] {
		return fmt.Errorf("status %q not in allowed set", status)
	}
	if len(deletedData) > MaxDataCategories {
		return fmt.Errorf("deleted_data has %d categories, max %d", len(deletedData), MaxDataCategories)
	}
	for _, c := range deletedData {
		if c == "" {
			return fmt.Errorf("deleted_data contains empty category")
		}
		if utf8.RuneCountInString(c) > MaxCategoryLen {
			return fmt.Errorf("category %q exceeds %d chars", c, MaxCategoryLen)
		}
		if !allowedDataCategories[c] {
			return fmt.Errorf("category %q not in allowed personal-data set", c)
		}
	}
	return nil
}
