package config

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
)

const encryptedPrefix = "enc:v1:"

// EncryptedValue wraps a value encrypted with AES-256-GCM.
// Format: "enc:v1:<base64-encoded-ciphertext>"
type EncryptedValue string

// IsEncrypted checks if a string is an encrypted value.
func IsEncrypted(s string) bool {
	return strings.HasPrefix(s, encryptedPrefix)
}

// Encrypt encrypts a plaintext value using AES-256-GCM.
// The key must be exactly 32 bytes.
func Encrypt(plaintext string, key []byte) (EncryptedValue, error) {
	if len(key) != 32 {
		return "", errors.New("encryption key must be 32 bytes")
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("create GCM: %w", err)
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", fmt.Errorf("generate nonce: %w", err)
	}

	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	encoded := base64.StdEncoding.EncodeToString(ciphertext)

	return EncryptedValue(encryptedPrefix + encoded), nil
}

// Decrypt decrypts an encrypted value.
func (v EncryptedValue) Decrypt(key []byte) (string, error) {
	if len(key) != 32 {
		return "", errors.New("encryption key must be 32 bytes")
	}

	s := string(v)
	if !strings.HasPrefix(s, encryptedPrefix) {
		return "", errors.New("value is not encrypted")
	}

	encoded := strings.TrimPrefix(s, encryptedPrefix)
	data, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", fmt.Errorf("decode base64: %w", err)
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("create GCM: %w", err)
	}

	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return "", errors.New("ciphertext too short")
	}

	nonce, ciphertext := data[:nonceSize], data[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", fmt.Errorf("decrypt: %w", err)
	}

	return string(plaintext), nil
}

// DecryptEnv decrypts all encrypted values in a map of environment variables.
// Non-encrypted values are passed through unchanged.
func DecryptEnv(env map[string]string, key []byte) (map[string]string, error) {
	result := make(map[string]string, len(env))
	for k, v := range env {
		if IsEncrypted(v) {
			decrypted, err := EncryptedValue(v).Decrypt(key)
			if err != nil {
				return nil, fmt.Errorf("decrypt %s: %w", k, err)
			}
			result[k] = decrypted
		} else {
			result[k] = v
		}
	}
	return result, nil
}
