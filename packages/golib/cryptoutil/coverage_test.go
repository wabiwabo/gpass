package cryptoutil

import (
	"strings"
	"testing"
)

// TestSecureRandom_NonPositive pins the n<=0 guard plus its propagation
// through SecureRandomHex/SecureRandomBase64/SecureToken.
func TestSecureRandom_NonPositive(t *testing.T) {
	for _, n := range []int{0, -1, -100} {
		if _, err := SecureRandom(n); err == nil {
			t.Errorf("SecureRandom(%d) err = nil", n)
		}
		if _, err := SecureRandomHex(n); err == nil {
			t.Errorf("SecureRandomHex(%d) err = nil", n)
		}
		if _, err := SecureRandomBase64(n); err == nil {
			t.Errorf("SecureRandomBase64(%d) err = nil", n)
		}
		if _, err := SecureToken(n); err == nil {
			t.Errorf("SecureToken(%d) err = nil", n)
		}
	}
}

// TestDeriveKey_Errors pins the master-key-too-short and bad-length guards.
func TestDeriveKey_Errors(t *testing.T) {
	if _, err := DeriveKey(make([]byte, 8), "ctx", 32); err == nil {
		t.Error("short master key accepted")
	}
	if _, err := DeriveKey(make([]byte, 16), "ctx", 0); err == nil {
		t.Error("zero length accepted")
	}
	if _, err := DeriveKey(make([]byte, 16), "ctx", 65); err == nil {
		t.Error("oversized length accepted")
	}
}

// TestDeriveKey_Sha512Branch pins that requesting >32 bytes triggers the
// SHA-512 fallback path.
func TestDeriveKey_Sha512Branch(t *testing.T) {
	mk := make([]byte, 32)
	for i := range mk {
		mk[i] = byte(i)
	}
	out, err := DeriveKey(mk, "purpose", 64)
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 64 {
		t.Errorf("len = %d, want 64", len(out))
	}
	// Determinism: same inputs → same output.
	out2, _ := DeriveKey(mk, "purpose", 64)
	for i := range out {
		if out[i] != out2[i] {
			t.Fatal("derive not deterministic")
		}
	}
	// Domain separation: different context → different output.
	other, _ := DeriveKey(mk, "other", 64)
	same := true
	for i := range out {
		if out[i] != other[i] {
			same = false
			break
		}
	}
	if same {
		t.Error("DeriveKey ignored context (no domain separation)")
	}
}

// TestHashHex_UnknownAlgorithm pins the error propagation from Hash.
func TestHashHex_UnknownAlgorithm(t *testing.T) {
	_, err := HashHex("md5", []byte("x"))
	if err == nil || !strings.Contains(err.Error(), "unsupported") {
		t.Errorf("err = %v, want unsupported", err)
	}
}
