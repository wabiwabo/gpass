package kms

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"sync"
	"time"
)

var (
	// ErrInvalidKey is returned when a key has invalid length.
	ErrInvalidKey = errors.New("kms: key must be 32 bytes")
	// ErrCiphertextTooShort is returned when ciphertext is too short to decode.
	ErrCiphertextTooShort = errors.New("kms: ciphertext too short")
	// ErrKeyNotFound is returned when a key version is not found.
	ErrKeyNotFound = errors.New("kms: key version not found")
	// ErrProviderUnavailable is returned when the key provider cannot be reached.
	ErrProviderUnavailable = errors.New("kms: provider unavailable")
)

// KeyProvider generates and wraps/unwraps data encryption keys (DEKs).
// Production: AWS KMS, HashiCorp Vault
// Development: LocalProvider
type KeyProvider interface {
	// GenerateDEK generates a new plaintext DEK and its encrypted form.
	GenerateDEK() (plaintext []byte, encrypted []byte, err error)
	// DecryptDEK decrypts an encrypted DEK using the KEK.
	DecryptDEK(encrypted []byte) ([]byte, error)
	// KeyID returns the identifier of the current KEK.
	KeyID() string
}

// LocalProvider implements KeyProvider using a local master key.
// Suitable for development and testing only.
type LocalProvider struct {
	masterKey []byte
	keyID     string
}

// NewLocalProvider creates a LocalProvider with the given 32-byte master key.
func NewLocalProvider(masterKey []byte, keyID string) (*LocalProvider, error) {
	if len(masterKey) != 32 {
		return nil, ErrInvalidKey
	}
	k := make([]byte, 32)
	copy(k, masterKey)
	return &LocalProvider{masterKey: k, keyID: keyID}, nil
}

// GenerateDEK generates a random 32-byte DEK and encrypts it with the master key.
func (p *LocalProvider) GenerateDEK() (plaintext []byte, encrypted []byte, err error) {
	dek := make([]byte, 32)
	if _, err := rand.Read(dek); err != nil {
		return nil, nil, fmt.Errorf("kms: generate DEK: %w", err)
	}

	block, err := aes.NewCipher(p.masterKey)
	if err != nil {
		return nil, nil, fmt.Errorf("kms: create cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, nil, fmt.Errorf("kms: create GCM: %w", err)
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, nil, fmt.Errorf("kms: generate nonce: %w", err)
	}

	encryptedDEK := gcm.Seal(nonce, nonce, dek, nil)
	return dek, encryptedDEK, nil
}

// DecryptDEK decrypts an encrypted DEK using the master key.
func (p *LocalProvider) DecryptDEK(encrypted []byte) ([]byte, error) {
	block, err := aes.NewCipher(p.masterKey)
	if err != nil {
		return nil, fmt.Errorf("kms: create cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("kms: create GCM: %w", err)
	}

	nonceSize := gcm.NonceSize()
	if len(encrypted) < nonceSize {
		return nil, ErrCiphertextTooShort
	}

	nonce, ciphertext := encrypted[:nonceSize], encrypted[nonceSize:]
	dek, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("kms: decrypt DEK: %w", err)
	}
	return dek, nil
}

// KeyID returns the provider's key identifier.
func (p *LocalProvider) KeyID() string { return p.keyID }

// EnvelopeEncryptor implements envelope encryption:
// each encryption generates a fresh DEK, encrypts data with DEK, wraps DEK with KEK.
// Output format: [2-byte key_id_len][key_id][encrypted_dek_len(4 bytes)][encrypted_dek][nonce][ciphertext+tag]
type EnvelopeEncryptor struct {
	provider KeyProvider
	cache    *dekCache
}

// NewEnvelopeEncryptor creates a new EnvelopeEncryptor with the given key provider.
func NewEnvelopeEncryptor(provider KeyProvider) *EnvelopeEncryptor {
	return &EnvelopeEncryptor{
		provider: provider,
		cache:    newDEKCache(5 * time.Minute),
	}
}

// Encrypt encrypts plaintext using envelope encryption.
// Returns base64-encoded ciphertext containing the encrypted DEK and data.
func (e *EnvelopeEncryptor) Encrypt(plaintext []byte) (string, error) {
	dek, encryptedDEK, err := e.provider.GenerateDEK()
	if err != nil {
		return "", err
	}

	block, err := aes.NewCipher(dek)
	if err != nil {
		return "", fmt.Errorf("kms: create data cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("kms: create data GCM: %w", err)
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", fmt.Errorf("kms: generate data nonce: %w", err)
	}

	ciphertext := gcm.Seal(nil, nonce, plaintext, nil)
	keyID := e.provider.KeyID()

	// Pack: [key_id_len(2)][key_id][enc_dek_len(4)][enc_dek][nonce][ciphertext]
	keyIDBytes := []byte(keyID)
	result := make([]byte, 0, 2+len(keyIDBytes)+4+len(encryptedDEK)+len(nonce)+len(ciphertext))

	// Key ID length (2 bytes, big endian)
	kidLen := make([]byte, 2)
	binary.BigEndian.PutUint16(kidLen, uint16(len(keyIDBytes)))
	result = append(result, kidLen...)
	result = append(result, keyIDBytes...)

	// Encrypted DEK length (4 bytes, big endian)
	dekLen := make([]byte, 4)
	binary.BigEndian.PutUint32(dekLen, uint32(len(encryptedDEK)))
	result = append(result, dekLen...)
	result = append(result, encryptedDEK...)

	// Nonce + ciphertext
	result = append(result, nonce...)
	result = append(result, ciphertext...)

	return base64.StdEncoding.EncodeToString(result), nil
}

// Decrypt decrypts base64-encoded envelope-encrypted data.
func (e *EnvelopeEncryptor) Decrypt(encoded string) ([]byte, error) {
	data, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, fmt.Errorf("kms: decode base64: %w", err)
	}

	if len(data) < 6 {
		return nil, ErrCiphertextTooShort
	}

	// Read key ID
	kidLen := binary.BigEndian.Uint16(data[:2])
	pos := 2
	if len(data) < pos+int(kidLen)+4 {
		return nil, ErrCiphertextTooShort
	}
	_ = string(data[pos : pos+int(kidLen)]) // keyID (reserved for multi-provider routing)
	pos += int(kidLen)

	// Read encrypted DEK
	dekLen := binary.BigEndian.Uint32(data[pos : pos+4])
	pos += 4
	if len(data) < pos+int(dekLen) {
		return nil, ErrCiphertextTooShort
	}
	encryptedDEK := data[pos : pos+int(dekLen)]
	pos += int(dekLen)

	// Decrypt DEK (with caching)
	cacheKey := base64.RawStdEncoding.EncodeToString(encryptedDEK)
	dek, err := e.cache.getOrDecrypt(cacheKey, func() ([]byte, error) {
		return e.provider.DecryptDEK(encryptedDEK)
	})
	if err != nil {
		return nil, err
	}

	// Decrypt data with DEK
	block, err := aes.NewCipher(dek)
	if err != nil {
		return nil, fmt.Errorf("kms: create data cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("kms: create data GCM: %w", err)
	}

	nonceSize := gcm.NonceSize()
	remaining := data[pos:]
	if len(remaining) < nonceSize {
		return nil, ErrCiphertextTooShort
	}

	nonce := remaining[:nonceSize]
	ciphertext := remaining[nonceSize:]

	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("kms: decrypt data: %w", err)
	}

	return plaintext, nil
}

// EncryptString is a convenience method for encrypting strings.
func (e *EnvelopeEncryptor) EncryptString(s string) (string, error) {
	return e.Encrypt([]byte(s))
}

// DecryptString is a convenience method for decrypting to string.
func (e *EnvelopeEncryptor) DecryptString(encoded string) (string, error) {
	b, err := e.Decrypt(encoded)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// dekCache caches decrypted DEKs to reduce KMS calls during decryption.
type dekCache struct {
	mu      sync.RWMutex
	entries map[string]dekCacheEntry
	ttl     time.Duration
}

type dekCacheEntry struct {
	dek       []byte
	expiresAt time.Time
}

func newDEKCache(ttl time.Duration) *dekCache {
	return &dekCache{
		entries: make(map[string]dekCacheEntry),
		ttl:     ttl,
	}
}

func (c *dekCache) getOrDecrypt(key string, decrypt func() ([]byte, error)) ([]byte, error) {
	c.mu.RLock()
	if entry, ok := c.entries[key]; ok && time.Now().Before(entry.expiresAt) {
		result := make([]byte, len(entry.dek))
		copy(result, entry.dek)
		c.mu.RUnlock()
		return result, nil
	}
	c.mu.RUnlock()

	dek, err := decrypt()
	if err != nil {
		return nil, err
	}

	cached := make([]byte, len(dek))
	copy(cached, dek)

	c.mu.Lock()
	c.entries[key] = dekCacheEntry{dek: cached, expiresAt: time.Now().Add(c.ttl)}
	c.mu.Unlock()

	result := make([]byte, len(dek))
	copy(result, dek)
	return result, nil
}

// ClearCache invalidates all cached DEKs. Called during key rotation.
func (e *EnvelopeEncryptor) ClearCache() {
	e.cache.mu.Lock()
	e.cache.entries = make(map[string]dekCacheEntry)
	e.cache.mu.Unlock()
}
