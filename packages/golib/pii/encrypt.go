package pii

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strings"
)

// Encryptor provides field-level encryption for PII data.
type Encryptor struct {
	key []byte // 32 bytes for AES-256
}

// NewEncryptor creates a PII encryptor with the given 32-byte key.
func NewEncryptor(key []byte) (*Encryptor, error) {
	if len(key) != 32 {
		return nil, fmt.Errorf("pii: key must be exactly 32 bytes, got %d", len(key))
	}
	k := make([]byte, 32)
	copy(k, key)
	return &Encryptor{key: k}, nil
}

// Encrypt encrypts plaintext using AES-256-GCM and returns base64-encoded ciphertext.
// Each encryption produces unique output (random nonce).
func (e *Encryptor) Encrypt(plaintext string) (string, error) {
	block, err := aes.NewCipher(e.key)
	if err != nil {
		return "", fmt.Errorf("pii: create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("pii: create GCM: %w", err)
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", fmt.Errorf("pii: generate nonce: %w", err)
	}

	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// Decrypt decrypts base64-encoded ciphertext.
func (e *Encryptor) Decrypt(ciphertext string) (string, error) {
	data, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return "", fmt.Errorf("pii: decode base64: %w", err)
	}

	block, err := aes.NewCipher(e.key)
	if err != nil {
		return "", fmt.Errorf("pii: create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("pii: create GCM: %w", err)
	}

	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return "", fmt.Errorf("pii: ciphertext too short")
	}

	nonce, encrypted := data[:nonceSize], data[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, encrypted, nil)
	if err != nil {
		return "", fmt.Errorf("pii: decrypt: %w", err)
	}

	return string(plaintext), nil
}

// EncryptFields encrypts multiple fields in a map. Returns a new map with encrypted values.
func (e *Encryptor) EncryptFields(fields map[string]string) (map[string]string, error) {
	result := make(map[string]string, len(fields))
	for k, v := range fields {
		encrypted, err := e.Encrypt(v)
		if err != nil {
			return nil, fmt.Errorf("pii: encrypt field %q: %w", k, err)
		}
		result[k] = encrypted
	}
	return result, nil
}

// DecryptFields decrypts multiple fields in a map.
func (e *Encryptor) DecryptFields(fields map[string]string) (map[string]string, error) {
	result := make(map[string]string, len(fields))
	for k, v := range fields {
		decrypted, err := e.Decrypt(v)
		if err != nil {
			return nil, fmt.Errorf("pii: decrypt field %q: %w", k, err)
		}
		result[k] = decrypted
	}
	return result, nil
}

// MaskField masks a string showing only the last N characters.
// "John Doe" with last=3 -> "****Doe"
func MaskField(value string, lastVisible int) string {
	if value == "" {
		return ""
	}
	runes := []rune(value)
	if lastVisible >= len(runes) {
		return value
	}
	masked := strings.Repeat("*", len(runes)-lastVisible)
	return masked + string(runes[len(runes)-lastVisible:])
}

// HashField creates a one-way hash of a field for lookup without decryption.
// Uses HMAC-SHA256 with the encryptor's key.
func (e *Encryptor) HashField(value string) string {
	mac := hmac.New(sha256.New, e.key)
	mac.Write([]byte(value))
	return hex.EncodeToString(mac.Sum(nil))
}
