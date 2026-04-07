package kms

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"strings"
	"testing"
)

func newProvider(t *testing.T) *LocalProvider {
	t.Helper()
	k := make([]byte, 32)
	if _, err := rand.Read(k); err != nil {
		t.Fatal(err)
	}
	p, err := NewLocalProvider(k, "test-key-v1")
	if err != nil {
		t.Fatal(err)
	}
	return p
}

// TestNewLocalProvider_RejectsBadLengths covers both directions of the
// 32-byte length guard.
func TestNewLocalProvider_RejectsBadLengths(t *testing.T) {
	for _, n := range []int{0, 16, 31, 33, 64} {
		_, err := NewLocalProvider(make([]byte, n), "k")
		if !errors.Is(err, ErrInvalidKey) {
			t.Errorf("len=%d: err = %v, want ErrInvalidKey", n, err)
		}
	}
}

// TestEnvelope_RoundTrip pins the canonical Encrypt → Decrypt loop and
// covers both EncryptString and DecryptString.
func TestEnvelope_RoundTrip(t *testing.T) {
	e := NewEnvelopeEncryptor(newProvider(t))

	cases := []string{
		"",
		"hello",
		"3171234567890001 — Indonesian NIK",
		strings.Repeat("x", 4096),
	}
	for _, plain := range cases {
		ct, err := e.EncryptString(plain)
		if err != nil {
			t.Fatalf("EncryptString(%q): %v", plain, err)
		}
		got, err := e.DecryptString(ct)
		if err != nil {
			t.Fatalf("DecryptString: %v", err)
		}
		if got != plain {
			t.Errorf("round-trip lost: got %q, want %q", got, plain)
		}
	}
}

// TestEnvelope_NonDeterministic pins that two encryptions of the same
// plaintext produce different ciphertexts (fresh DEK + nonce per call).
// Critical for the same reason as iter 65's pii test: a deterministic
// AEAD would leak inference attacks.
func TestEnvelope_NonDeterministic(t *testing.T) {
	e := NewEnvelopeEncryptor(newProvider(t))
	a, _ := e.Encrypt([]byte("same"))
	b, _ := e.Encrypt([]byte("same"))
	if a == b {
		t.Error("two encryptions of the same plaintext produced identical ciphertext")
	}
}

// TestDecrypt_BadBase64 covers the base64 decode error.
func TestDecrypt_BadBase64(t *testing.T) {
	e := NewEnvelopeEncryptor(newProvider(t))
	_, err := e.Decrypt("!!!not base64!!!")
	if err == nil || !strings.Contains(err.Error(), "decode base64") {
		t.Errorf("err = %v", err)
	}
}

// TestDecrypt_TooShortAtEachStage pins the four ErrCiphertextTooShort
// branches in Decrypt: header (<6), key-id-section, dek-section, and
// remaining-after-pos<nonceSize.
func TestDecrypt_TooShortAtEachStage(t *testing.T) {
	e := NewEnvelopeEncryptor(newProvider(t))

	// 5 bytes < 6 minimum header.
	tiny := base64.StdEncoding.EncodeToString([]byte{0, 0, 0, 0, 0})
	if _, err := e.Decrypt(tiny); !errors.Is(err, ErrCiphertextTooShort) {
		t.Errorf("5-byte: err = %v", err)
	}

	// Header says key-id is 100 bytes long but buffer is much shorter.
	header := []byte{0, 100, 'a', 'b', 'c', 'd'}
	if _, err := e.Decrypt(base64.StdEncoding.EncodeToString(header)); !errors.Is(err, ErrCiphertextTooShort) {
		t.Errorf("oversized kid: err = %v", err)
	}
}

// TestDecrypt_TamperedCiphertextRejected pins the AEAD authentication
// guarantee on the *data* layer (not the DEK layer): a single bit-flip
// in the encrypted payload must cause Decrypt to fail.
func TestDecrypt_TamperedCiphertextRejected(t *testing.T) {
	e := NewEnvelopeEncryptor(newProvider(t))
	ct, _ := e.EncryptString("sensitive")

	raw, _ := base64.StdEncoding.DecodeString(ct)
	// Flip the last byte (which is part of the GCM tag).
	raw[len(raw)-1] ^= 0xff
	tampered := base64.StdEncoding.EncodeToString(raw)

	_, err := e.Decrypt(tampered)
	if err == nil {
		t.Fatal("tampered ciphertext was accepted")
	}
}

// TestEnvelope_DEKCacheReusesDecryption pins that a second decrypt of
// the same ciphertext doesn't re-call the provider's DecryptDEK. We
// instrument by wrapping LocalProvider in a counting decorator.
func TestEnvelope_DEKCacheReusesDecryption(t *testing.T) {
	inner := newProvider(t)
	counter := &countingProvider{inner: inner}
	e := NewEnvelopeEncryptor(counter)

	ct, _ := e.EncryptString("hello")
	if counter.decryptCalls != 0 {
		t.Errorf("baseline DecryptDEK calls = %d, want 0", counter.decryptCalls)
	}

	// First decrypt: cache miss → 1 provider call.
	if _, err := e.DecryptString(ct); err != nil {
		t.Fatal(err)
	}
	if counter.decryptCalls != 1 {
		t.Errorf("after 1st decrypt: calls = %d, want 1", counter.decryptCalls)
	}

	// Second decrypt of the same ciphertext: cache hit → still 1 call.
	if _, err := e.DecryptString(ct); err != nil {
		t.Fatal(err)
	}
	if counter.decryptCalls != 1 {
		t.Errorf("after 2nd decrypt: calls = %d, want 1 (cache miss)", counter.decryptCalls)
	}

	// ClearCache forces the next decrypt to hit the provider again.
	e.ClearCache()
	if _, err := e.DecryptString(ct); err != nil {
		t.Fatal(err)
	}
	if counter.decryptCalls != 2 {
		t.Errorf("after clear+decrypt: calls = %d, want 2", counter.decryptCalls)
	}
}

// countingProvider wraps a LocalProvider and counts DecryptDEK calls.
type countingProvider struct {
	inner        *LocalProvider
	decryptCalls int
}

func (p *countingProvider) GenerateDEK() ([]byte, []byte, error) { return p.inner.GenerateDEK() }
func (p *countingProvider) DecryptDEK(b []byte) ([]byte, error) {
	p.decryptCalls++
	return p.inner.DecryptDEK(b)
}
func (p *countingProvider) KeyID() string { return p.inner.KeyID() }

// TestLocalProvider_DecryptDEK_CiphertextTooShort covers the nonceSize
// guard in LocalProvider.DecryptDEK.
func TestLocalProvider_DecryptDEK_CiphertextTooShort(t *testing.T) {
	p := newProvider(t)
	_, err := p.DecryptDEK([]byte{1, 2, 3}) // 3 bytes < 12-byte nonce
	if !errors.Is(err, ErrCiphertextTooShort) {
		t.Errorf("err = %v, want ErrCiphertextTooShort", err)
	}
}
