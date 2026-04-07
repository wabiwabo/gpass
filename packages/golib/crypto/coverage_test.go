package crypto

import (
	"bytes"
	"crypto/rand"
	"strings"
	"testing"
)

// newKey returns a fresh 32-byte AES-256 key for tests.
func newKey(t *testing.T) []byte {
	t.Helper()
	k := make([]byte, 32)
	if _, err := rand.Read(k); err != nil {
		t.Fatal(err)
	}
	return k
}

// TestKeyRing_RotateRoundTripAcrossVersions covers the multi-version
// decrypt path: encrypt under v1, rotate, encrypt under v2, then decrypt
// both with the same KeyRing. v1 ciphertexts must continue to decrypt
// after rotation — this is the entire point of having a key ring.
func TestKeyRing_RotateRoundTripAcrossVersions(t *testing.T) {
	k1 := newKey(t)
	kr, err := NewKeyRing(k1)
	if err != nil {
		t.Fatal(err)
	}

	ct1, err := kr.Encrypt("payload-v1")
	if err != nil {
		t.Fatal(err)
	}
	if kr.ActiveVersion() != 1 {
		t.Errorf("ActiveVersion = %d, want 1", kr.ActiveVersion())
	}

	// Rotate. Old keys must remain in the ring.
	if err := kr.Rotate(newKey(t)); err != nil {
		t.Fatal(err)
	}
	if kr.ActiveVersion() != 2 {
		t.Errorf("ActiveVersion after rotate = %d, want 2", kr.ActiveVersion())
	}

	ct2, err := kr.Encrypt("payload-v2")
	if err != nil {
		t.Fatal(err)
	}

	// Both round-trip.
	pt1, err := kr.Decrypt(ct1)
	if err != nil || pt1 != "payload-v1" {
		t.Errorf("v1 round-trip: pt=%q err=%v", pt1, err)
	}
	pt2, err := kr.Decrypt(ct2)
	if err != nil || pt2 != "payload-v2" {
		t.Errorf("v2 round-trip: pt=%q err=%v", pt2, err)
	}

	// Versions() lists both, no key bytes leaked.
	infos := kr.Versions()
	if len(infos) != 2 {
		t.Errorf("Versions = %d entries, want 2", len(infos))
	}
	for _, info := range infos {
		if info.Version == 0 {
			t.Errorf("zero-value version in info: %+v", info)
		}
	}
}

// TestKeyRing_DecryptForeignCiphertextFailsAllKeys covers the
// "lastErr from all keys" branch — a ciphertext that no key in the
// ring can decrypt must surface "decrypt failed with all keys".
func TestKeyRing_DecryptForeignCiphertextFailsAllKeys(t *testing.T) {
	kr, _ := NewKeyRing(newKey(t))

	// Create a SECOND ring with a different key, encrypt there, then try
	// to decrypt with the first ring.
	other, _ := NewKeyRing(newKey(t))
	foreign, _ := other.Encrypt("not for you")

	_, err := kr.Decrypt(foreign)
	if err == nil {
		t.Fatal("expected decrypt failure")
	}
	if !strings.Contains(err.Error(), "decrypt failed with all keys") {
		t.Errorf("err = %q", err.Error())
	}
}

// TestKeyRing_DecryptInvalidBase64 covers the base64 decode error branch.
func TestKeyRing_DecryptInvalidBase64(t *testing.T) {
	kr, _ := NewKeyRing(newKey(t))
	_, err := kr.Decrypt("!!!not-base64!!!")
	if err == nil || !strings.Contains(err.Error(), "decode base64") {
		t.Errorf("err = %v", err)
	}
}

// TestKeyRing_DecryptTooShort covers the length<2 guard.
func TestKeyRing_DecryptTooShort(t *testing.T) {
	kr, _ := NewKeyRing(newKey(t))
	// "AA==" decodes to a single zero byte → len(data)=1, fails the guard.
	_, err := kr.Decrypt("AA==")
	if err == nil || !strings.Contains(err.Error(), "ciphertext too short") {
		t.Errorf("err = %v", err)
	}
}

// TestKeyRing_BadKeyLengthRejected covers NewKeyRing and Rotate length
// validation.
func TestKeyRing_BadKeyLengthRejected(t *testing.T) {
	if _, err := NewKeyRing([]byte("too-short")); err == nil {
		t.Error("NewKeyRing with 9-byte key should fail")
	}
	kr, _ := NewKeyRing(newKey(t))
	if err := kr.Rotate([]byte{1, 2, 3}); err == nil {
		t.Error("Rotate with 3-byte key should fail")
	}
}

// TestKeyRing_DecryptInternallyTriedMatchingVersionFirst pins that the
// version-byte hint causes the matching key to be tried first. We can't
// observe ordering directly, but we can ensure the round-trip still
// works after several rotations (4+ keys), which exercises the
// "ordered: matching first, others after" branch.
func TestKeyRing_RoundTripAfterMultipleRotations(t *testing.T) {
	kr, _ := NewKeyRing(newKey(t))

	cts := make([]string, 0, 5)
	for i := 0; i < 5; i++ {
		ct, err := kr.Encrypt("msg")
		if err != nil {
			t.Fatal(err)
		}
		cts = append(cts, ct)
		if i < 4 {
			if err := kr.Rotate(newKey(t)); err != nil {
				t.Fatal(err)
			}
		}
	}

	if kr.ActiveVersion() != 5 {
		t.Errorf("ActiveVersion = %d, want 5", kr.ActiveVersion())
	}

	for i, ct := range cts {
		pt, err := kr.Decrypt(ct)
		if err != nil {
			t.Errorf("ct[%d]: %v", i, err)
			continue
		}
		if pt != "msg" {
			t.Errorf("ct[%d] decrypted to %q", i, pt)
		}
	}
}

// TestGenerateRandomBytes_HexDeterminism pins that the hex helper is
// twice the byte length and that two consecutive calls return distinct
// values (catches a "I cached the result" regression).
func TestGenerateRandomHex_DistinctAndCorrectLength(t *testing.T) {
	a, err := GenerateRandomHex(16)
	if err != nil {
		t.Fatal(err)
	}
	b, err := GenerateRandomHex(16)
	if err != nil {
		t.Fatal(err)
	}
	if len(a) != 32 || len(b) != 32 {
		t.Errorf("hex length = %d / %d, want 32 each (16 bytes)", len(a), len(b))
	}
	if a == b {
		t.Error("two consecutive GenerateRandomHex calls returned identical strings")
	}
}

// TestGenerateRandomBytes_RequestedLength covers the bytes helper.
func TestGenerateRandomBytes_RequestedLength(t *testing.T) {
	b, err := GenerateRandomBytes(48)
	if err != nil {
		t.Fatal(err)
	}
	if len(b) != 48 {
		t.Errorf("len = %d, want 48", len(b))
	}
	zero := make([]byte, 48)
	if bytes.Equal(b, zero) {
		t.Error("48 random bytes returned all-zero — RNG is broken or test is fake")
	}
}
