// Package valueguard provides value validation guards that return
// typed errors. Validates inputs at system boundaries with clear,
// machine-readable error messages.
package valueguard

import (
	"fmt"
	"net"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"
)

// ValidationError represents a validation failure.
type ValidationError struct {
	Field   string `json:"field"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("%s: %s", e.Field, e.Message)
}

func fail(field, code, message string) *ValidationError {
	return &ValidationError{Field: field, Code: code, Message: message}
}

// Required checks that a string is not empty.
func Required(field, value string) *ValidationError {
	if strings.TrimSpace(value) == "" {
		return fail(field, "required", field+" is required")
	}
	return nil
}

// MinLength checks minimum string length in runes.
func MinLength(field, value string, min int) *ValidationError {
	if utf8.RuneCountInString(value) < min {
		return fail(field, "min_length", fmt.Sprintf("%s must be at least %d characters", field, min))
	}
	return nil
}

// MaxLength checks maximum string length in runes.
func MaxLength(field, value string, max int) *ValidationError {
	if utf8.RuneCountInString(value) > max {
		return fail(field, "max_length", fmt.Sprintf("%s must be at most %d characters", field, max))
	}
	return nil
}

// Range checks that a number is within bounds.
func Range(field string, value, min, max int) *ValidationError {
	if value < min || value > max {
		return fail(field, "out_of_range", fmt.Sprintf("%s must be between %d and %d", field, min, max))
	}
	return nil
}

// Positive checks that a number is positive.
func Positive(field string, value int) *ValidationError {
	if value <= 0 {
		return fail(field, "must_be_positive", field+" must be positive")
	}
	return nil
}

var emailRe = regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)

// Email validates an email address format.
func Email(field, value string) *ValidationError {
	if !emailRe.MatchString(value) {
		return fail(field, "invalid_email", field+" is not a valid email")
	}
	return nil
}

// OneOf checks that a value is one of the allowed values.
func OneOf(field, value string, allowed ...string) *ValidationError {
	for _, a := range allowed {
		if value == a {
			return nil
		}
	}
	return fail(field, "invalid_value", fmt.Sprintf("%s must be one of: %s", field, strings.Join(allowed, ", ")))
}

// NotFuture checks that a time is not in the future.
func NotFuture(field string, value time.Time) *ValidationError {
	if value.After(time.Now()) {
		return fail(field, "future_date", field+" cannot be in the future")
	}
	return nil
}

// NotPast checks that a time is not in the past.
func NotPast(field string, value time.Time) *ValidationError {
	if value.Before(time.Now()) {
		return fail(field, "past_date", field+" cannot be in the past")
	}
	return nil
}

// IP validates an IP address.
func IP(field, value string) *ValidationError {
	if net.ParseIP(value) == nil {
		return fail(field, "invalid_ip", field+" is not a valid IP address")
	}
	return nil
}

// Matches checks a value against a regex pattern.
func Matches(field, value, pattern string) *ValidationError {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return fail(field, "invalid_pattern", "invalid validation pattern")
	}
	if !re.MatchString(value) {
		return fail(field, "pattern_mismatch", field+" does not match required pattern")
	}
	return nil
}

// Validate runs multiple guards and returns the first error.
func Validate(guards ...*ValidationError) *ValidationError {
	for _, g := range guards {
		if g != nil {
			return g
		}
	}
	return nil
}

// ValidateAll runs all guards and returns all errors.
func ValidateAll(guards ...*ValidationError) []*ValidationError {
	var errs []*ValidationError
	for _, g := range guards {
		if g != nil {
			errs = append(errs, g)
		}
	}
	return errs
}
