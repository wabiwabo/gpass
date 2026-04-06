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
// Must be exactly 16 digits with province code between 11 and 94.
func NIKFormat(nik string) error {
	if !nikRegex.MatchString(nik) {
		return fmt.Errorf("NIK must be exactly 16 digits")
	}
	province := (nik[0]-'0')*10 + (nik[1] - '0')
	if province < 11 || province > 94 {
		return fmt.Errorf("NIK has invalid province code: %d", province)
	}
	// Validate district code (digits 2-3): 01-99.
	district := (nik[2]-'0')*10 + (nik[3] - '0')
	if district < 1 {
		return fmt.Errorf("NIK has invalid district code: %02d", district)
	}
	// Validate sub-district code (digits 4-5): 01-99.
	subdistrict := (nik[4]-'0')*10 + (nik[5] - '0')
	if subdistrict < 1 {
		return fmt.Errorf("NIK has invalid sub-district code: %02d", subdistrict)
	}
	// Birth date encoding (digits 6-11): DDMMYY
	// Female NIK: day += 40 (so 41-71 for females).
	day := (nik[6]-'0')*10 + (nik[7] - '0')
	month := (nik[8]-'0')*10 + (nik[9] - '0')
	if day == 0 || (day > 31 && day < 41) || day > 71 {
		return fmt.Errorf("NIK has invalid birth date encoding")
	}
	if month < 1 || month > 12 {
		return fmt.Errorf("NIK has invalid birth month: %02d", month)
	}
	return nil
}

// NPWPFormat validates an Indonesian NPWP (Nomor Pokok Wajib Pajak).
// Accepts both formatted (XX.XXX.XXX.X-XXX.XXX) and raw 15-digit formats.
func NPWPFormat(npwp string) error {
	// Strip separators.
	var digits strings.Builder
	for _, r := range npwp {
		if r >= '0' && r <= '9' {
			digits.WriteRune(r)
		} else if r != '.' && r != '-' {
			return fmt.Errorf("NPWP contains invalid character: %c", r)
		}
	}
	d := digits.String()

	if len(d) != 15 {
		return fmt.Errorf("NPWP must be exactly 15 digits, got %d", len(d))
	}

	// First 2 digits are tax type code (01-99).
	taxType := (d[0]-'0')*10 + (d[1] - '0')
	if taxType < 1 {
		return fmt.Errorf("NPWP has invalid tax type code: %02d", taxType)
	}

	return nil
}

// PhoneIDFormat validates an Indonesian phone number.
// Must start with +62 and have 8-13 digits after the country code.
func PhoneIDFormat(phone string) error {
	if !strings.HasPrefix(phone, "+62") {
		return fmt.Errorf("phone must start with +62 (Indonesian format)")
	}
	digits := phone[3:]
	if len(digits) < 8 || len(digits) > 13 {
		return fmt.Errorf("phone must have 8-13 digits after +62, got %d", len(digits))
	}
	for _, c := range digits {
		if c < '0' || c > '9' {
			return fmt.Errorf("phone must contain only digits after country code")
		}
	}
	return nil
}

// NIBFormat validates an Indonesian NIB (Nomor Induk Berusaha).
// NIB is a 13-digit business identification number.
func NIBFormat(nib string) error {
	if len(nib) != 13 {
		return fmt.Errorf("NIB must be exactly 13 digits, got %d", len(nib))
	}
	for _, c := range nib {
		if c < '0' || c > '9' {
			return fmt.Errorf("NIB must contain only digits")
		}
	}
	return nil
}

// AKTAFormat validates an Indonesian company deed number (Akta Pendirian).
// Format varies but typically numeric, 1-10 digits.
func AKTAFormat(akta string) error {
	if len(akta) == 0 {
		return fmt.Errorf("akta number is required")
	}
	if len(akta) > 20 {
		return fmt.Errorf("akta number must not exceed 20 characters")
	}
	// Allow digits, hyphens, and slashes (format varies by notary).
	for _, c := range akta {
		if !((c >= '0' && c <= '9') || c == '-' || c == '/') {
			return fmt.Errorf("akta number contains invalid character: %c", c)
		}
	}
	return nil
}

// SKFormat validates an Indonesian legal entity registration number (SK Kemenkumham).
// Format: AHU-XXXXX.AH.XX.XX
func SKFormat(sk string) error {
	if len(sk) == 0 {
		return fmt.Errorf("SK number is required")
	}
	if !strings.HasPrefix(sk, "AHU-") {
		return fmt.Errorf("SK number must start with AHU-")
	}
	return nil
}

// IsAlpha checks that value contains only ASCII letters.
func IsAlpha(name, value string) error {
	for _, c := range value {
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')) {
			return fmt.Errorf("%s must contain only letters", name)
		}
	}
	return nil
}

// IsAlphanumeric checks that value contains only ASCII letters and digits.
func IsAlphanumeric(name, value string) error {
	for _, c := range value {
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9')) {
			return fmt.Errorf("%s must contain only letters and digits", name)
		}
	}
	return nil
}

// IsNumeric checks that value contains only digits.
func IsNumeric(name, value string) error {
	for _, c := range value {
		if c < '0' || c > '9' {
			return fmt.Errorf("%s must contain only digits", name)
		}
	}
	return nil
}

// InRange checks that a numeric value is within bounds.
func InRange(name string, value, min, max int) error {
	if value < min || value > max {
		return fmt.Errorf("%s must be between %d and %d", name, min, max)
	}
	return nil
}

// OneOf checks that value is one of the allowed options.
func OneOf(name, value string, allowed []string) error {
	for _, a := range allowed {
		if value == a {
			return nil
		}
	}
	return fmt.Errorf("%s must be one of: %s", name, strings.Join(allowed, ", "))
}
