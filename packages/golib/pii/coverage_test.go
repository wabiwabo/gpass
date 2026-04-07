package pii

import (
	"crypto/rand"
	"strings"
	"testing"
)

func freshKey(t *testing.T) []byte {
	t.Helper()
	k := make([]byte, 32)
	if _, err := rand.Read(k); err != nil {
		t.Fatal(err)
	}
	return k
}

// TestEncrypt_NonDeterministic pins that two encryptions of the same
// plaintext produce different ciphertexts (random nonce per call).
// A deterministic AEAD would leak whether two PII fields are equal —
// catastrophic for "do these two records belong to the same NIK?"
// inference attacks.
func TestEncrypt_NonDeterministic(t *testing.T) {
	e, err := NewEncryptor(freshKey(t))
	if err != nil {
		t.Fatal(err)
	}
	a, err := e.Encrypt("3171234567890001") // mock NIK
	if err != nil {
		t.Fatal(err)
	}
	b, err := e.Encrypt("3171234567890001")
	if err != nil {
		t.Fatal(err)
	}
	if a == b {
		t.Error("two encryptions of the same plaintext returned identical ciphertext — nonce reuse!")
	}
	// Both must still decrypt to the same plaintext.
	pa, _ := e.Decrypt(a)
	pb, _ := e.Decrypt(b)
	if pa != pb || pa != "3171234567890001" {
		t.Errorf("round-trip lost: a=%q b=%q", pa, pb)
	}
}

// TestDecrypt_TamperedCiphertextRejected pins the AEAD authentication
// guarantee: a single bit-flip in the ciphertext must cause Decrypt
// to fail (not silently return garbage). This is the entire point of
// using GCM over CTR.
func TestDecrypt_TamperedCiphertextRejected(t *testing.T) {
	e, _ := NewEncryptor(freshKey(t))
	ct, _ := e.Encrypt("sensitive")
	// Flip one byte in the middle of the base64 string. We have to
	// flip *bytes* of the underlying ciphertext, not base64 chars,
	// otherwise we just hit the base64-decode error path.
	raw, _ := decodeForTest(ct)
	raw[len(raw)/2] ^= 0xff
	tampered := encodeForTest(raw)
	_, err := e.Decrypt(tampered)
	if err == nil {
		t.Fatal("tampered ciphertext was accepted — AEAD authentication is broken")
	}
	if !strings.Contains(err.Error(), "decrypt") {
		t.Errorf("err = %q", err.Error())
	}
}

// TestDecrypt_BadBase64 covers the base64 decode error branch.
func TestDecrypt_BadBase64(t *testing.T) {
	e, _ := NewEncryptor(freshKey(t))
	_, err := e.Decrypt("!!!not base64!!!")
	if err == nil || !strings.Contains(err.Error(), "decode base64") {
		t.Errorf("err = %v", err)
	}
}

// TestDecrypt_TooShort covers the len<nonceSize guard.
func TestDecrypt_TooShort_Cov(t *testing.T) {
	e, _ := NewEncryptor(freshKey(t))
	// Two bytes is far below GCM nonce size (12).
	_, err := e.Decrypt("AAA=")
	if err == nil || !strings.Contains(err.Error(), "ciphertext too short") {
		t.Errorf("err = %v", err)
	}
}

// TestEncryptDecryptFields_RoundTripAndError covers the EncryptFields
// happy path plus the DecryptFields error wrap when one field is
// corrupt — the per-field error must include the field name so
// operators can locate the bad row.
func TestEncryptDecryptFields_RoundTripAndError(t *testing.T) {
	e, _ := NewEncryptor(freshKey(t))
	plain := map[string]string{
		"nik":  "3171234567890001",
		"name": "Budi Santoso",
		"npwp": "01.234.567.8-901.000",
	}
	enc, err := e.EncryptFields(plain)
	if err != nil {
		t.Fatal(err)
	}
	if len(enc) != 3 {
		t.Fatalf("encrypted map has %d entries, want 3", len(enc))
	}
	for k := range plain {
		if enc[k] == plain[k] {
			t.Errorf("field %q was not encrypted", k)
		}
	}

	// Round-trip.
	dec, err := e.DecryptFields(enc)
	if err != nil {
		t.Fatal(err)
	}
	for k, v := range plain {
		if dec[k] != v {
			t.Errorf("field %q round-trip: got %q, want %q", k, dec[k], v)
		}
	}

	// Corrupt one field; the error must name the field.
	enc["nik"] = "garbage"
	_, err = e.DecryptFields(enc)
	if err == nil || !strings.Contains(err.Error(), `field "nik"`) {
		t.Errorf("corrupt field error = %v, want substring 'field \"nik\"'", err)
	}
}

// TestHashField_DeterministicAcrossKeys pins that HashField is:
// (1) deterministic for the same key+value (so it can be used as a
// lookup index), and (2) different across different keys (so dumping
// one keyspace's hashes doesn't leak inferences about another).
func TestHashField_DeterministicAcrossKeys(t *testing.T) {
	k1 := freshKey(t)
	k2 := freshKey(t)
	e1, _ := NewEncryptor(k1)
	e2, _ := NewEncryptor(k2)

	h1a := e1.HashField("3171234567890001")
	h1b := e1.HashField("3171234567890001")
	if h1a != h1b {
		t.Error("HashField is non-deterministic for the same key+value")
	}

	h2 := e2.HashField("3171234567890001")
	if h1a == h2 {
		t.Error("HashField returned the same hash under two different keys — keyed hashing is broken")
	}
}

// TestMaskField_AllBranches pins each branch of MaskField, including
// "lastVisible >= len(runes)" → return whole string, multibyte runes.
func TestMaskField_AllBranches(t *testing.T) {
	if got := MaskField("", 3); got != "" {
		t.Errorf("empty string = %q", got)
	}
	if got := MaskField("Budi", 3); got != "*udi" {
		t.Errorf("Budi(3) = %q, want *udi", got)
	}
	if got := MaskField("Budi", 100); got != "Budi" {
		t.Errorf("Budi(100) should return whole string, got %q", got)
	}
	// Multibyte: "東京" is 2 runes / 6 UTF-8 bytes. last=1 → "*京".
	if got := MaskField("東京", 1); got != "*京" {
		t.Errorf("multibyte mask = %q, want *京", got)
	}
}

// TestNewEncryptor_RejectsBadLengths pins both length-validation paths.
func TestNewEncryptor_RejectsBadLengths(t *testing.T) {
	for _, n := range []int{0, 16, 31, 33, 64} {
		_, err := NewEncryptor(make([]byte, n))
		if err == nil {
			t.Errorf("NewEncryptor with %d-byte key should fail", n)
		}
	}
}

// helpers — keep base64 dependency local to the test file to avoid
// shadowing the production import.
func decodeForTest(s string) ([]byte, error) { return base64StdDecodeString(s) }
func encodeForTest(b []byte) string          { return base64StdEncodeToString(b) }
