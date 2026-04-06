package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"sync"
	"time"
)

// KeyVersion represents a versioned encryption key.
type KeyVersion struct {
	Version   int
	Key       []byte // 32 bytes for AES-256
	CreatedAt time.Time
	Active    bool // only one active at a time
}

// KeyVersionInfo exposes key metadata without the actual key bytes.
type KeyVersionInfo struct {
	Version   int       `json:"version"`
	Active    bool      `json:"active"`
	CreatedAt time.Time `json:"created_at"`
}

// KeyRing manages multiple key versions for seamless rotation.
// Encrypt always uses the active key. Decrypt tries all keys.
type KeyRing struct {
	keys []KeyVersion
	mu   sync.RWMutex
}

// NewKeyRing creates a new KeyRing with the given initial key.
// The key must be exactly 32 bytes for AES-256.
func NewKeyRing(initialKey []byte) (*KeyRing, error) {
	if len(initialKey) != 32 {
		return nil, fmt.Errorf("crypto: key must be 32 bytes, got %d", len(initialKey))
	}

	keyCopy := make([]byte, 32)
	copy(keyCopy, initialKey)

	kr := &KeyRing{
		keys: []KeyVersion{
			{
				Version:   1,
				Key:       keyCopy,
				CreatedAt: time.Now().UTC(),
				Active:    true,
			},
		},
	}
	return kr, nil
}

// Encrypt encrypts plaintext with the active key using AES-256-GCM.
// The ciphertext is prefixed with a version byte and base64-encoded.
func (kr *KeyRing) Encrypt(plaintext string) (string, error) {
	kr.mu.RLock()
	var activeKey KeyVersion
	found := false
	for _, k := range kr.keys {
		if k.Active {
			activeKey = k
			found = true
			break
		}
	}
	kr.mu.RUnlock()

	if !found {
		return "", fmt.Errorf("crypto: no active key found")
	}

	block, err := aes.NewCipher(activeKey.Key)
	if err != nil {
		return "", fmt.Errorf("crypto: create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("crypto: create GCM: %w", err)
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", fmt.Errorf("crypto: generate nonce: %w", err)
	}

	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)

	// Prepend version byte
	versioned := make([]byte, 1+len(ciphertext))
	versioned[0] = byte(activeKey.Version)
	copy(versioned[1:], ciphertext)

	return base64.StdEncoding.EncodeToString(versioned), nil
}

// Decrypt decodes base64 ciphertext, reads the version byte, and tries
// the matching key first. If that fails, it tries all other keys.
func (kr *KeyRing) Decrypt(ciphertext string) (string, error) {
	data, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return "", fmt.Errorf("crypto: decode base64: %w", err)
	}

	if len(data) < 2 {
		return "", fmt.Errorf("crypto: ciphertext too short")
	}

	version := int(data[0])
	encData := data[1:]

	kr.mu.RLock()
	keys := make([]KeyVersion, len(kr.keys))
	copy(keys, kr.keys)
	kr.mu.RUnlock()

	// Try the matching version first, then all others
	ordered := make([]KeyVersion, 0, len(keys))
	for _, k := range keys {
		if k.Version == version {
			ordered = append([]KeyVersion{k}, ordered...)
		} else {
			ordered = append(ordered, k)
		}
	}

	var lastErr error
	for _, k := range ordered {
		plaintext, err := decryptWithKey(k.Key, encData)
		if err != nil {
			lastErr = err
			continue
		}
		return plaintext, nil
	}

	if lastErr != nil {
		return "", fmt.Errorf("crypto: decrypt failed with all keys: %w", lastErr)
	}
	return "", fmt.Errorf("crypto: no keys available")
}

func decryptWithKey(key, data []byte) (string, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return "", fmt.Errorf("ciphertext shorter than nonce")
	}

	nonce, ciphertext := data[:nonceSize], data[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", err
	}

	return string(plaintext), nil
}

// Rotate adds a new key and makes it active. Old keys are kept for decryption.
func (kr *KeyRing) Rotate(newKey []byte) error {
	if len(newKey) != 32 {
		return fmt.Errorf("crypto: key must be 32 bytes, got %d", len(newKey))
	}

	keyCopy := make([]byte, 32)
	copy(keyCopy, newKey)

	kr.mu.Lock()
	defer kr.mu.Unlock()

	// Deactivate all existing keys
	for i := range kr.keys {
		kr.keys[i].Active = false
	}

	nextVersion := kr.keys[len(kr.keys)-1].Version + 1
	kr.keys = append(kr.keys, KeyVersion{
		Version:   nextVersion,
		Key:       keyCopy,
		CreatedAt: time.Now().UTC(),
		Active:    true,
	})

	return nil
}

// ActiveVersion returns the current active key version.
func (kr *KeyRing) ActiveVersion() int {
	kr.mu.RLock()
	defer kr.mu.RUnlock()

	for _, k := range kr.keys {
		if k.Active {
			return k.Version
		}
	}
	return 0
}

// Versions returns all key versions without the actual key bytes.
func (kr *KeyRing) Versions() []KeyVersionInfo {
	kr.mu.RLock()
	defer kr.mu.RUnlock()

	infos := make([]KeyVersionInfo, len(kr.keys))
	for i, k := range kr.keys {
		infos[i] = KeyVersionInfo{
			Version:   k.Version,
			Active:    k.Active,
			CreatedAt: k.CreatedAt,
		}
	}
	return infos
}
