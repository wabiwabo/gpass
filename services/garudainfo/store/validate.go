package store

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

// Enterprise validation limits for consent records (UU PDP No. 27/2022).
const (
	MaxUserIDLen     = 128
	MaxClientIDLen   = 100
	MaxClientNameLen = 255
	MaxPurposeLen    = 255
	MaxFieldKeys     = 50
	MaxFieldKeyLen   = 64
	MinDuration      = int64(1)             // 1 second (sub-minute used in tests/short-lived flows)
	MaxDuration      = int64(5 * 365 * 86400) // 5 years
)

// allowedFieldKeys is the closed set of personal data fields that can be
// consented to. Any new field requires an explicit code change + DPO approval.
var allowedFieldKeys = map[string]bool{
	"name":          true,
	"nik":           true,
	"dob":           true,
	"birth_place":   true,
	"address":       true,
	"phone":         true,
	"email":         true,
	"gender":        true,
	"marital":       true,
	"religion":      true,
	"occupation":    true,
	"nationality":   true,
	"family":        true,
	"photo":         true,
	"signature":     true,
	"biometric":     true,
	"npwp":          true,
	"education":     true,
	"blood_type":    true,
}

// ValidateConsent enforces required fields, length bounds, duration limits,
// and the field-key allow-list. Called by both InMemory and Postgres impls.
func ValidateConsent(c *Consent) error {
	if c == nil {
		return fmt.Errorf("consent is nil")
	}
	if err := requireBounded("user_id", c.UserID, MaxUserIDLen); err != nil {
		return err
	}
	if err := requireBounded("client_id", c.ClientID, MaxClientIDLen); err != nil {
		return err
	}
	if err := requireBounded("client_name", c.ClientName, MaxClientNameLen); err != nil {
		return err
	}
	if err := requireBounded("purpose", c.Purpose, MaxPurposeLen); err != nil {
		return err
	}
	if c.DurationSeconds < MinDuration {
		return fmt.Errorf("duration_seconds %d below minimum %d", c.DurationSeconds, MinDuration)
	}
	if c.DurationSeconds > MaxDuration {
		return fmt.Errorf("duration_seconds %d exceeds max %d (5 years)", c.DurationSeconds, MaxDuration)
	}
	if len(c.Fields) == 0 {
		return fmt.Errorf("fields is required (at least one consented field)")
	}
	if len(c.Fields) > MaxFieldKeys {
		return fmt.Errorf("fields has %d keys, max %d", len(c.Fields), MaxFieldKeys)
	}
	hasGranted := false
	for k, v := range c.Fields {
		if k == "" {
			return fmt.Errorf("field key is empty")
		}
		if utf8.RuneCountInString(k) > MaxFieldKeyLen {
			return fmt.Errorf("field key %q exceeds %d chars", k, MaxFieldKeyLen)
		}
		if !allowedFieldKeys[k] {
			return fmt.Errorf("field key %q not in allowed personal-data set", k)
		}
		if v {
			hasGranted = true
		}
	}
	if !hasGranted {
		return fmt.Errorf("fields must grant at least one field (all values false)")
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
