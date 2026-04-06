package validate

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

var (
	emailRegex = regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)
	uuidRegex  = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-4[0-9a-fA-F]{3}-[89abAB][0-9a-fA-F]{3}-[0-9a-fA-F]{12}$`)
	nikRegex   = regexp.MustCompile(`^\d{16}$`)
)

// Errors collects multiple validation errors.
type Errors struct {
	errs []error
}

// Add appends an error if it is non-nil.
func (e *Errors) Add(err error) {
	if err != nil {
		e.errs = append(e.errs, err)
	}
}

// HasErrors returns true if any validation errors were collected.
func (e *Errors) HasErrors() bool {
	return len(e.errs) > 0
}

// Error implements the error interface.
func (e *Errors) Error() string {
	if len(e.errs) == 0 {
		return ""
	}
	msgs := make([]string, len(e.errs))
	for i, err := range e.errs {
		msgs[i] = err.Error()
	}
	return strings.Join(msgs, "; ")
}

// All returns all collected errors.
func (e *Errors) All() []error {
	return e.errs
}

// Required checks that value is non-empty.
func Required(name, value string) error {
	if strings.TrimSpace(value) == "" {
		return fmt.Errorf("%s is required", name)
	}
	return nil
}

// MinLength checks that value has at least min characters.
func MinLength(name, value string, min int) error {
	if len(value) < min {
		return fmt.Errorf("%s must be at least %d characters", name, min)
	}
	return nil
}

// MaxLength checks that value has at most max characters.
func MaxLength(name, value string, max int) error {
	if len(value) > max {
		return fmt.Errorf("%s must be at most %d characters", name, max)
	}
	return nil
}

// IsEmail checks that value is a valid email format.
func IsEmail(name, value string) error {
	if !emailRegex.MatchString(value) {
		return fmt.Errorf("%s must be a valid email address", name)
	}
	return nil
}

// IsURL checks that value is a valid URL.
func IsURL(name, value string) error {
	if _, err := url.ParseRequestURI(value); err != nil {
		return fmt.Errorf("%s must be a valid URL", name)
	}
	return nil
}

// IsUUID checks that value is a valid UUID v4.
func IsUUID(name, value string) error {
	if !uuidRegex.MatchString(value) {
		return fmt.Errorf("%s must be a valid UUID v4", name)
	}
	return nil
}

// NIKFormat validates an Indonesian NIK (Nomor Induk Kependudukan).
// Must be exactly 16 digits with province code between 11 and 99.
func NIKFormat(nik string) error {
	if !nikRegex.MatchString(nik) {
		return fmt.Errorf("NIK must be exactly 16 digits")
	}
	province := (nik[0]-'0')*10 + (nik[1] - '0')
	if province < 11 || province > 99 {
		return fmt.Errorf("NIK has invalid province code: %d", province)
	}
	return nil
}
