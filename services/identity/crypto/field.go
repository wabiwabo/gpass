package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"fmt"
	"io"
)

// FieldEncryptor implements envelope encryption with DEK/KEK pattern.
type FieldEncryptor struct {
	kekCipher cipher.AEAD
}

// NewFieldEncryptor creates a new FieldEncryptor with the given Key Encryption Key.
// KEK must be exactly 32 bytes for AES-256.
func NewFieldEncryptor(kek []byte) (*FieldEncryptor, error) {
	if len(kek) != 32 {
		return nil, fmt.Errorf("KEK must be 32 bytes, got %d", len(kek))
	}
	block, err := aes.NewCipher(kek)
	if err != nil {
		return nil, fmt.Errorf("create AES cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("create GCM: %w", err)
	}
	return &FieldEncryptor{kekCipher: gcm}, nil
}

// GenerateWrappedDEK generates a random 32-byte Data Encryption Key and wraps it
// with the KEK using AES-256-GCM.
func (fe *FieldEncryptor) GenerateWrappedDEK() ([]byte, error) {
	dek := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, dek); err != nil {
		return nil, fmt.Errorf("generate DEK: %w", err)
	}
	nonce := make([]byte, fe.kekCipher.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("generate nonce: %w", err)
	}
	// Wrapped DEK = nonce || ciphertext (with GCM tag)
	wrapped := fe.kekCipher.Seal(nonce, nonce, dek, nil)
	return wrapped, nil
}

// unwrapDEK decrypts a wrapped DEK using the KEK.
func (fe *FieldEncryptor) unwrapDEK(wrappedDEK []byte) ([]byte, error) {
	nonceSize := fe.kekCipher.NonceSize()
	if len(wrappedDEK) < nonceSize {
		return nil, fmt.Errorf("wrapped DEK too short")
	}
	nonce := wrappedDEK[:nonceSize]
	ciphertext := wrappedDEK[nonceSize:]
	dek, err := fe.kekCipher.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("unwrap DEK: %w", err)
	}
	return dek, nil
}

// EncryptField encrypts plaintext using the DEK (unwrapped from wrappedDEK).
func (fe *FieldEncryptor) EncryptField(wrappedDEK, plaintext []byte) ([]byte, error) {
	dek, err := fe.unwrapDEK(wrappedDEK)
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(dek)
	if err != nil {
		return nil, fmt.Errorf("create DEK cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("create DEK GCM: %w", err)
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("generate field nonce: %w", err)
	}
	return gcm.Seal(nonce, nonce, plaintext, nil), nil
}

// DecryptField decrypts ciphertext using the DEK (unwrapped from wrappedDEK).
func (fe *FieldEncryptor) DecryptField(wrappedDEK, ciphertext []byte) ([]byte, error) {
	dek, err := fe.unwrapDEK(wrappedDEK)
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(dek)
	if err != nil {
		return nil, fmt.Errorf("create DEK cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("create DEK GCM: %w", err)
	}
	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, fmt.Errorf("ciphertext too short")
	}
	nonce := ciphertext[:nonceSize]
	ct := ciphertext[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ct, nil)
	if err != nil {
		return nil, fmt.Errorf("decrypt field: %w", err)
	}
	return plaintext, nil
}
