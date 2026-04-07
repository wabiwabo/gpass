package token

import (
	"regexp"
	"strings"
	"testing"
)

// TestGenerateAPIKey_EnvValidation pins the env allowlist (test|live).
func TestGenerateAPIKey_EnvValidation(t *testing.T) {
	for _, bad := range []string{"", "prod", "dev", "LIVE", "Test"} {
		if _, err := GenerateAPIKey(bad, 32); err == nil {
			t.Errorf("env=%q accepted", bad)
		}
	}
	for _, ok := range []string{"test", "live"} {
		k, err := GenerateAPIKey(ok, 32)
		if err != nil {
			t.Errorf("env=%q: %v", ok, err)
		}
		if !strings.HasPrefix(k, "gp_"+ok+"_") {
			t.Errorf("missing prefix: %q", k)
		}
	}
}

// TestGenerateAPIKey_DefaultLength pins that tokenLen<16 falls back to 32.
func TestGenerateAPIKey_DefaultLength(t *testing.T) {
	k, err := GenerateAPIKey("live", 4)
	if err != nil {
		t.Fatal(err)
	}
	// "gp_live_" prefix is 8 chars, then 32 base62 chars = 40 total.
	if len(k) != 8+32 {
		t.Errorf("len = %d, want 40 (%q)", len(k), k)
	}
}

// TestGenerateOTP_Bounds pins the 4-10 digit guard.
func TestGenerateOTP_Bounds(t *testing.T) {
	for _, bad := range []int{0, 1, 3, 11, 100, -1} {
		if _, err := GenerateOTP(bad); err == nil {
			t.Errorf("digits=%d accepted", bad)
		}
	}
	for _, n := range []int{4, 6, 8, 10} {
		otp, err := GenerateOTP(n)
		if err != nil {
			t.Fatalf("digits=%d: %v", n, err)
		}
		if len(otp) != n {
			t.Errorf("digits=%d: got len %d", n, len(otp))
		}
		for _, c := range otp {
			if c < '0' || c > '9' {
				t.Errorf("non-digit in %q", otp)
				break
			}
		}
	}
}

// TestGenerateUUID_V4Format pins RFC 4122 v4 layout: version nibble = 4
// and variant nibble in {8,9,a,b}.
func TestGenerateUUID_V4Format(t *testing.T) {
	re := regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)
	seen := make(map[string]bool)
	for i := 0; i < 50; i++ {
		u, err := GenerateUUID()
		if err != nil {
			t.Fatal(err)
		}
		if !re.MatchString(u) {
			t.Errorf("not v4: %q", u)
		}
		if seen[u] {
			t.Errorf("duplicate UUID: %q", u)
		}
		seen[u] = true
	}
}

// TestGenerateBase62_LengthAndAlphabet pins the exact char-length contract
// and that every emitted character is in the base62 alphabet.
func TestGenerateBase62_LengthAndAlphabet(t *testing.T) {
	s, err := GenerateBase62(64)
	if err != nil {
		t.Fatal(err)
	}
	if len(s) != 64 {
		t.Errorf("len = %d, want 64", len(s))
	}
	for _, c := range s {
		if !strings.ContainsRune(base62Alphabet, c) {
			t.Errorf("non-base62 char %q in %q", c, s)
		}
	}
	// Zero-length is a valid edge case.
	z, err := GenerateBase62(0)
	if err != nil || z != "" {
		t.Errorf("zero len: %q %v", z, err)
	}
}

// TestGenerate_LengthInvariants pins the hex/base64url/raw byte length
// contracts. These are easy invariants but they're the contract callers
// rely on for sizing storage and headers.
func TestGenerate_LengthInvariants(t *testing.T) {
	for _, n := range []int{1, 16, 32, 64} {
		hexTok, _ := Generate(n)
		if len(hexTok) != 2*n {
			t.Errorf("Generate(%d) hex len = %d, want %d", n, len(hexTok), 2*n)
		}
		b64, _ := GenerateBase64URL(n)
		// RawURLEncoding length: ceil(n*4/3) without padding.
		if strings.ContainsAny(b64, "=+/") {
			t.Errorf("base64url contains padding/non-url chars: %q", b64)
		}
		raw, _ := Bytes(n)
		if len(raw) != n {
			t.Errorf("Bytes(%d) len = %d", n, len(raw))
		}
	}
}
