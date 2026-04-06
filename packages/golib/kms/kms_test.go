package kms

import (
	"crypto/rand"
	"fmt"
	"strings"
	"sync"
	"testing"
)

func testMasterKey(t *testing.T) []byte {
	t.Helper()
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		t.Fatal(err)
	}
	return key
}

func TestLocalProvider_GenerateDEK(t *testing.T) {
	p, err := NewLocalProvider(testMasterKey(t), "test-key-1")
	if err != nil {
		t.Fatal(err)
	}

	plaintext, encrypted, err := p.GenerateDEK()
	if err != nil {
		t.Fatal(err)
	}

	if len(plaintext) != 32 {
		t.Errorf("DEK length: got %d, want 32", len(plaintext))
	}
	if len(encrypted) == 0 {
		t.Error("encrypted DEK should not be empty")
	}

	// Encrypted DEK should be different from plaintext.
	if string(plaintext) == string(encrypted) {
		t.Error("encrypted DEK should differ from plaintext")
	}
}

func TestLocalProvider_DecryptDEK(t *testing.T) {
	p, err := NewLocalProvider(testMasterKey(t), "test-key-1")
	if err != nil {
		t.Fatal(err)
	}

	plaintext, encrypted, err := p.GenerateDEK()
	if err != nil {
		t.Fatal(err)
	}

	decrypted, err := p.DecryptDEK(encrypted)
	if err != nil {
		t.Fatal(err)
	}

	if string(decrypted) != string(plaintext) {
		t.Error("decrypted DEK should match original plaintext")
	}
}

func TestLocalProvider_DecryptDEK_WrongKey(t *testing.T) {
	key1 := testMasterKey(t)
	key2 := testMasterKey(t)

	p1, _ := NewLocalProvider(key1, "key1")
	p2, _ := NewLocalProvider(key2, "key2")

	_, encrypted, _ := p1.GenerateDEK()
	_, err := p2.DecryptDEK(encrypted)
	if err == nil {
		t.Error("should fail to decrypt with wrong master key")
	}
}

func TestLocalProvider_DecryptDEK_TooShort(t *testing.T) {
	p, _ := NewLocalProvider(testMasterKey(t), "k")
	_, err := p.DecryptDEK([]byte{1, 2, 3})
	if err == nil {
		t.Error("should fail on short ciphertext")
	}
}

func TestLocalProvider_InvalidKeyLength(t *testing.T) {
	_, err := NewLocalProvider([]byte("short"), "k")
	if err == nil {
		t.Error("should reject non-32-byte key")
	}
}

func TestLocalProvider_KeyID(t *testing.T) {
	p, _ := NewLocalProvider(testMasterKey(t), "my-key-v2")
	if p.KeyID() != "my-key-v2" {
		t.Errorf("KeyID: got %q, want %q", p.KeyID(), "my-key-v2")
	}
}

func TestLocalProvider_UniqueDEKs(t *testing.T) {
	p, _ := NewLocalProvider(testMasterKey(t), "k")

	seen := make(map[string]bool)
	for i := 0; i < 50; i++ {
		dek, _, err := p.GenerateDEK()
		if err != nil {
			t.Fatal(err)
		}
		s := string(dek)
		if seen[s] {
			t.Fatal("generated duplicate DEK")
		}
		seen[s] = true
	}
}

func TestEnvelopeEncryptor_EncryptDecrypt(t *testing.T) {
	p, _ := NewLocalProvider(testMasterKey(t), "test-key")
	enc := NewEnvelopeEncryptor(p)

	plaintext := "Hello, GarudaPass! NIK: 3201120509870001"

	ciphertext, err := enc.EncryptString(plaintext)
	if err != nil {
		t.Fatal(err)
	}

	if ciphertext == plaintext {
		t.Error("ciphertext should differ from plaintext")
	}

	decrypted, err := enc.DecryptString(ciphertext)
	if err != nil {
		t.Fatal(err)
	}

	if decrypted != plaintext {
		t.Errorf("decrypted: got %q, want %q", decrypted, plaintext)
	}
}

func TestEnvelopeEncryptor_EncryptDecrypt_Bytes(t *testing.T) {
	p, _ := NewLocalProvider(testMasterKey(t), "test-key")
	enc := NewEnvelopeEncryptor(p)

	data := []byte{0x00, 0xFF, 0x80, 0x7F, 0x01}

	ciphertext, err := enc.Encrypt(data)
	if err != nil {
		t.Fatal(err)
	}

	decrypted, err := enc.Decrypt(ciphertext)
	if err != nil {
		t.Fatal(err)
	}

	if string(decrypted) != string(data) {
		t.Errorf("decrypted bytes don't match")
	}
}

func TestEnvelopeEncryptor_UniquePerEncryption(t *testing.T) {
	p, _ := NewLocalProvider(testMasterKey(t), "test-key")
	enc := NewEnvelopeEncryptor(p)

	c1, _ := enc.EncryptString("same data")
	c2, _ := enc.EncryptString("same data")

	if c1 == c2 {
		t.Error("same plaintext should produce different ciphertext (unique DEK + nonce)")
	}
}

func TestEnvelopeEncryptor_EmptyPlaintext(t *testing.T) {
	p, _ := NewLocalProvider(testMasterKey(t), "k")
	enc := NewEnvelopeEncryptor(p)

	ct, err := enc.EncryptString("")
	if err != nil {
		t.Fatal(err)
	}

	pt, err := enc.DecryptString(ct)
	if err != nil {
		t.Fatal(err)
	}
	if pt != "" {
		t.Errorf("expected empty string, got %q", pt)
	}
}

func TestEnvelopeEncryptor_LargePlaintext(t *testing.T) {
	p, _ := NewLocalProvider(testMasterKey(t), "k")
	enc := NewEnvelopeEncryptor(p)

	large := strings.Repeat("A", 10*1024*1024) // 10 MB
	ct, err := enc.EncryptString(large)
	if err != nil {
		t.Fatal(err)
	}
	pt, err := enc.DecryptString(ct)
	if err != nil {
		t.Fatal(err)
	}
	if pt != large {
		t.Error("large plaintext round-trip failed")
	}
}

func TestEnvelopeEncryptor_TamperedCiphertext(t *testing.T) {
	p, _ := NewLocalProvider(testMasterKey(t), "k")
	enc := NewEnvelopeEncryptor(p)

	ct, _ := enc.EncryptString("secret")

	// Tamper with the last byte.
	tampered := ct[:len(ct)-2] + "AA"

	_, err := enc.DecryptString(tampered)
	if err == nil {
		t.Error("should fail on tampered ciphertext")
	}
}

func TestEnvelopeEncryptor_InvalidBase64(t *testing.T) {
	p, _ := NewLocalProvider(testMasterKey(t), "k")
	enc := NewEnvelopeEncryptor(p)

	_, err := enc.DecryptString("not-valid-base64!!!")
	if err == nil {
		t.Error("should fail on invalid base64")
	}
}

func TestEnvelopeEncryptor_TooShortCiphertext(t *testing.T) {
	p, _ := NewLocalProvider(testMasterKey(t), "k")
	enc := NewEnvelopeEncryptor(p)

	// Very short base64 data.
	_, err := enc.Decrypt("AQID") // 3 bytes
	if err == nil {
		t.Error("should fail on too-short ciphertext")
	}
}

func TestEnvelopeEncryptor_ConcurrentAccess(t *testing.T) {
	p, _ := NewLocalProvider(testMasterKey(t), "concurrent")
	enc := NewEnvelopeEncryptor(p)

	var wg sync.WaitGroup
	errs := make(chan error, 100)

	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			data := strings.Repeat("x", n+1)
			ct, err := enc.EncryptString(data)
			if err != nil {
				errs <- err
				return
			}
			pt, err := enc.DecryptString(ct)
			if err != nil {
				errs <- err
				return
			}
			if pt != data {
				errs <- errorf("round-trip failed for length %d", n+1)
			}
		}(i)
	}
	wg.Wait()
	close(errs)

	for err := range errs {
		t.Error(err)
	}
}

func TestEnvelopeEncryptor_ClearCache(t *testing.T) {
	p, _ := NewLocalProvider(testMasterKey(t), "cache-test")
	enc := NewEnvelopeEncryptor(p)

	ct, _ := enc.EncryptString("cached")

	// First decrypt populates cache.
	pt1, err := enc.DecryptString(ct)
	if err != nil {
		t.Fatal(err)
	}

	// Clear cache.
	enc.ClearCache()

	// Second decrypt should still work (re-decrypts DEK from provider).
	pt2, err := enc.DecryptString(ct)
	if err != nil {
		t.Fatal(err)
	}

	if pt1 != pt2 || pt1 != "cached" {
		t.Error("cache clear should not affect decryption")
	}
}

func TestEnvelopeEncryptor_DEKCacheHit(t *testing.T) {
	p, _ := NewLocalProvider(testMasterKey(t), "cache")
	enc := NewEnvelopeEncryptor(p)

	ct, _ := enc.EncryptString("test")

	// Decrypt twice — second should use cache.
	for i := 0; i < 3; i++ {
		pt, err := enc.DecryptString(ct)
		if err != nil {
			t.Fatalf("decrypt %d: %v", i, err)
		}
		if pt != "test" {
			t.Errorf("decrypt %d: got %q", i, pt)
		}
	}
}

func errorf(format string, args ...interface{}) error {
	return fmt.Errorf(format, args...)
}
