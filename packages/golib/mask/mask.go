package mask

import (
	"strings"
	"unicode/utf8"
)

// NIK masks a 16-digit Indonesian NIK showing only the last 4 digits.
// "3201120509870001" → "************0001"
func NIK(nik string) string {
	if len(nik) != 16 {
		return strings.Repeat("*", len(nik))
	}
	return strings.Repeat("*", 12) + nik[12:]
}

// Email masks an email showing first char and domain.
// "john@example.com" → "j***@example.com"
func Email(email string) string {
	at := strings.LastIndex(email, "@")
	if at <= 0 {
		return strings.Repeat("*", len(email))
	}
	return string(email[0]) + strings.Repeat("*", at-1) + email[at:]
}

// Phone masks a phone number showing only last 4 digits.
// "+6281234567890" → "+62********7890"
func Phone(phone string) string {
	r := []rune(phone)
	if len(r) < 5 {
		return strings.Repeat("*", len(r))
	}
	visible := 4
	prefix := ""
	if strings.HasPrefix(phone, "+") {
		prefix = string(r[:3]) // keep country code
		masked := strings.Repeat("*", len(r)-3-visible)
		return prefix + masked + string(r[len(r)-visible:])
	}
	masked := strings.Repeat("*", len(r)-visible)
	return masked + string(r[len(r)-visible:])
}

// Name masks a name showing first and last character.
// "John Doe" → "J****** e"
func Name(name string) string {
	r := []rune(name)
	if len(r) <= 2 {
		return strings.Repeat("*", len(r))
	}
	return string(r[0]) + strings.Repeat("*", len(r)-2) + string(r[len(r)-1])
}

// CreditCard masks a credit card number showing only last 4 digits.
// "4111111111111111" → "************1111"
func CreditCard(cc string) string {
	digits := strings.ReplaceAll(cc, " ", "")
	digits = strings.ReplaceAll(digits, "-", "")
	if len(digits) < 4 {
		return strings.Repeat("*", len(digits))
	}
	return strings.Repeat("*", len(digits)-4) + digits[len(digits)-4:]
}

// NPWP masks an Indonesian NPWP showing only last 3 characters.
// "01.234.567.8-901.000" → "*****************000"
func NPWP(npwp string) string {
	if utf8.RuneCountInString(npwp) < 3 {
		return strings.Repeat("*", utf8.RuneCountInString(npwp))
	}
	r := []rune(npwp)
	return strings.Repeat("*", len(r)-3) + string(r[len(r)-3:])
}

// Partial masks a string showing the first `show` characters.
// "secret_data" with show=3 → "sec********"
func Partial(s string, show int) string {
	r := []rune(s)
	if show >= len(r) {
		return s
	}
	return string(r[:show]) + strings.Repeat("*", len(r)-show)
}

// Full replaces the entire string with asterisks.
func Full(s string) string {
	return strings.Repeat("*", utf8.RuneCountInString(s))
}

// Map masks values in a map based on field name patterns.
// Fields matching sensitive patterns are masked using the appropriate masker.
func Map(data map[string]string, sensitiveFields map[string]func(string) string) map[string]string {
	result := make(map[string]string, len(data))
	for k, v := range data {
		if masker, ok := sensitiveFields[k]; ok {
			result[k] = masker(v)
		} else {
			result[k] = v
		}
	}
	return result
}
