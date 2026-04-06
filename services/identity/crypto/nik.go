package crypto

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"regexp"
	"strconv"
)

var nikRegex = regexp.MustCompile(`^\d{16}$`)

// ValidateNIKFormat checks that a NIK is 16 digits with a valid province code (11-99).
func ValidateNIKFormat(nik string) error {
	if !nikRegex.MatchString(nik) {
		return fmt.Errorf("NIK must be exactly 16 digits")
	}
	provinceCode, _ := strconv.Atoi(nik[:2])
	if provinceCode < 11 || provinceCode > 99 {
		return fmt.Errorf("invalid province code: %d", provinceCode)
	}
	return nil
}

// TokenizeNIK produces a deterministic, non-reversible HMAC-SHA256 token from a NIK.
// The returned value is a 64-character hex string.
func TokenizeNIK(nik string, key []byte) string {
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(nik))
	return hex.EncodeToString(mac.Sum(nil))
}

// MaskNIK returns the NIK with all but the last 4 digits replaced by asterisks.
func MaskNIK(nik string) string {
	if len(nik) <= 4 {
		return nik
	}
	masked := ""
	for i := 0; i < len(nik)-4; i++ {
		masked += "*"
	}
	return masked + nik[len(nik)-4:]
}
