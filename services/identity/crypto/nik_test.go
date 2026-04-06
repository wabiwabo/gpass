package crypto

import (
	"testing"
)

func TestValidateNIKFormat_Valid(t *testing.T) {
	// Province code 32 (West Java)
	if err := ValidateNIKFormat("3201234567890001"); err != nil {
		t.Errorf("expected valid NIK, got error: %v", err)
	}
}

func TestValidateNIKFormat_TooShort(t *testing.T) {
	if err := ValidateNIKFormat("320123456789"); err == nil {
		t.Error("expected error for short NIK")
	}
}

func TestValidateNIKFormat_NonDigits(t *testing.T) {
	if err := ValidateNIKFormat("32012345678900ab"); err == nil {
		t.Error("expected error for non-digit NIK")
	}
}

func TestValidateNIKFormat_InvalidProvinceCode(t *testing.T) {
	// Province code 09 — invalid (below 11)
	if err := ValidateNIKFormat("0901234567890001"); err == nil {
		t.Error("expected error for province code < 11")
	}
}

func TestTokenizeNIK_Deterministic(t *testing.T) {
	key := []byte("01234567890123456789012345678901") // 32 bytes
	nik := "3201234567890001"

	t1 := TokenizeNIK(nik, key)
	t2 := TokenizeNIK(nik, key)

	if t1 != t2 {
		t.Errorf("tokenize not deterministic: %q != %q", t1, t2)
	}
	if len(t1) != 64 {
		t.Errorf("token length = %d, want 64", len(t1))
	}
}

func TestTokenizeNIK_DifferentNIKs(t *testing.T) {
	key := []byte("01234567890123456789012345678901")
	t1 := TokenizeNIK("3201234567890001", key)
	t2 := TokenizeNIK("3201234567890002", key)

	if t1 == t2 {
		t.Error("different NIKs should produce different tokens")
	}
}

func TestMaskNIK(t *testing.T) {
	masked := MaskNIK("3201234567890001")
	want := "************0001"
	if masked != want {
		t.Errorf("MaskNIK = %q, want %q", masked, want)
	}
}
