package session

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"regexp"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

var (
	ErrSessionNotFound = errors.New("session not found")
	ErrInvalidID       = errors.New("invalid session ID format")

	// Session IDs are 64-char hex strings (32 bytes of entropy).
	sessionIDPattern = regexp.MustCompile(`^[0-9a-f]{64}$`)
)

// Data holds the session payload stored in Redis (encrypted at rest).
type Data struct {
	UserID       string    `json:"user_id"`
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	IDToken      string    `json:"id_token,omitempty"`
	ExpiresAt    time.Time `json:"expires_at"`
	CSRFToken    string    `json:"csrf_token"`
	CreatedAt    time.Time `json:"created_at"`
	UserAgent    string    `json:"user_agent,omitempty"`
}

// Store defines the session persistence interface.
type Store interface {
	Create(ctx context.Context, data *Data, ttl time.Duration) (string, error)
	Get(ctx context.Context, sessionID string) (*Data, error)
	Update(ctx context.Context, sessionID string, data *Data, ttl time.Duration) error
	Delete(ctx context.Context, sessionID string) error
}

func generateID() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// ValidateID checks that a session ID has the expected format
// to prevent injection attacks via crafted cookie values.
func ValidateID(id string) error {
	if !sessionIDPattern.MatchString(id) {
		return ErrInvalidID
	}
	return nil
}

// --- Redis implementation with AES-GCM encryption ---

// RedisStore persists sessions in Redis with AES-256-GCM encryption at rest.
// This ensures that even if Redis is compromised, session tokens (access/refresh)
// cannot be extracted without the encryption key.
type RedisStore struct {
	client *redis.Client
	prefix string
	gcm    cipher.AEAD
}

// NewRedisStore creates a session store backed by Redis.
// encryptionKey must be exactly 32 bytes (256 bits) for AES-256.
// Pass nil to disable encryption (not recommended for production).
func NewRedisStore(client *redis.Client, encryptionKey []byte) (*RedisStore, error) {
	store := &RedisStore{client: client, prefix: "gpass:session:"}

	if len(encryptionKey) > 0 {
		if len(encryptionKey) != 32 {
			return nil, fmt.Errorf("encryption key must be exactly 32 bytes, got %d", len(encryptionKey))
		}
		block, err := aes.NewCipher(encryptionKey)
		if err != nil {
			return nil, fmt.Errorf("creating AES cipher: %w", err)
		}
		gcm, err := cipher.NewGCM(block)
		if err != nil {
			return nil, fmt.Errorf("creating GCM: %w", err)
		}
		store.gcm = gcm
	}

	return store, nil
}

// Encrypt encrypts data using AES-256-GCM. Exported for testing.
func (s *RedisStore) Encrypt(plaintext []byte) ([]byte, error) {
	return s.encrypt(plaintext)
}

// Decrypt decrypts data using AES-256-GCM. Exported for testing.
func (s *RedisStore) Decrypt(ciphertext []byte) ([]byte, error) {
	return s.decrypt(ciphertext)
}

func (s *RedisStore) encrypt(plaintext []byte) ([]byte, error) {
	if s.gcm == nil {
		return plaintext, nil
	}
	nonce := make([]byte, s.gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}
	return s.gcm.Seal(nonce, nonce, plaintext, nil), nil
}

func (s *RedisStore) decrypt(ciphertext []byte) ([]byte, error) {
	if s.gcm == nil {
		return ciphertext, nil
	}
	nonceSize := s.gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, errors.New("ciphertext too short")
	}
	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]
	return s.gcm.Open(nil, nonce, ciphertext, nil)
}

func (s *RedisStore) Create(ctx context.Context, data *Data, ttl time.Duration) (string, error) {
	sid, err := generateID()
	if err != nil {
		return "", err
	}
	if data.CreatedAt.IsZero() {
		data.CreatedAt = time.Now()
	}
	b, err := json.Marshal(data)
	if err != nil {
		return "", err
	}
	encrypted, err := s.encrypt(b)
	if err != nil {
		return "", fmt.Errorf("encrypting session data: %w", err)
	}
	if err := s.client.Set(ctx, s.prefix+sid, encrypted, ttl).Err(); err != nil {
		return "", err
	}
	return sid, nil
}

func (s *RedisStore) Get(ctx context.Context, sessionID string) (*Data, error) {
	if err := ValidateID(sessionID); err != nil {
		return nil, ErrSessionNotFound
	}
	raw, err := s.client.Get(ctx, s.prefix+sessionID).Bytes()
	if errors.Is(err, redis.Nil) {
		return nil, ErrSessionNotFound
	}
	if err != nil {
		return nil, err
	}
	b, err := s.decrypt(raw)
	if err != nil {
		return nil, fmt.Errorf("decrypting session data: %w", err)
	}
	var data Data
	if err := json.Unmarshal(b, &data); err != nil {
		return nil, err
	}
	return &data, nil
}

func (s *RedisStore) Update(ctx context.Context, sessionID string, data *Data, ttl time.Duration) error {
	if err := ValidateID(sessionID); err != nil {
		return err
	}
	b, err := json.Marshal(data)
	if err != nil {
		return err
	}
	encrypted, err := s.encrypt(b)
	if err != nil {
		return fmt.Errorf("encrypting session data: %w", err)
	}
	return s.client.Set(ctx, s.prefix+sessionID, encrypted, ttl).Err()
}

func (s *RedisStore) Delete(ctx context.Context, sessionID string) error {
	if err := ValidateID(sessionID); err != nil {
		return nil // silently ignore invalid IDs on delete
	}
	return s.client.Del(ctx, s.prefix+sessionID).Err()
}

// --- In-memory implementation (for testing) ---

type InMemoryStore struct {
	mu       sync.RWMutex
	sessions map[string]*Data
}

func NewInMemoryStore() *InMemoryStore {
	return &InMemoryStore{sessions: make(map[string]*Data)}
}

func (s *InMemoryStore) Create(_ context.Context, data *Data, _ time.Duration) (string, error) {
	sid, err := generateID()
	if err != nil {
		return "", err
	}
	if data.CreatedAt.IsZero() {
		data.CreatedAt = time.Now()
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	cp := *data
	s.sessions[sid] = &cp
	return sid, nil
}

func (s *InMemoryStore) Get(_ context.Context, sessionID string) (*Data, error) {
	if err := ValidateID(sessionID); err != nil {
		return nil, ErrSessionNotFound
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	d, ok := s.sessions[sessionID]
	if !ok {
		return nil, ErrSessionNotFound
	}
	cp := *d
	return &cp, nil
}

func (s *InMemoryStore) Update(_ context.Context, sessionID string, data *Data, _ time.Duration) error {
	if err := ValidateID(sessionID); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	cp := *data
	s.sessions[sessionID] = &cp
	return nil
}

func (s *InMemoryStore) Delete(_ context.Context, sessionID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.sessions, sessionID)
	return nil
}
