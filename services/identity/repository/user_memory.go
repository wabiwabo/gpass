package repository

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync"
	"time"
)

// InMemoryUserRepository implements UserRepository for testing.
type InMemoryUserRepository struct {
	mu    sync.RWMutex
	users map[string]*User        // keyed by ID
	byNIK map[string]string       // nik_token -> ID
}

// NewInMemoryUserRepository creates a new in-memory user repository.
func NewInMemoryUserRepository() *InMemoryUserRepository {
	return &InMemoryUserRepository{
		users: make(map[string]*User),
		byNIK: make(map[string]string),
	}
}

func (r *InMemoryUserRepository) Create(_ context.Context, user *User) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.byNIK[user.NIKToken]; exists {
		return ErrDuplicateNIKToken
	}

	id, err := generateUUID()
	if err != nil {
		return fmt.Errorf("generate id: %w", err)
	}

	now := time.Now().UTC()
	user.ID = id
	user.CreatedAt = now
	user.UpdatedAt = now

	stored := copyUser(user)
	r.users[user.ID] = stored
	r.byNIK[user.NIKToken] = user.ID

	return nil
}

func (r *InMemoryUserRepository) GetByID(_ context.Context, id string) (*User, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	u, ok := r.users[id]
	if !ok {
		return nil, ErrNotFound
	}
	return copyUser(u), nil
}

func (r *InMemoryUserRepository) GetByNIKToken(_ context.Context, nikToken string) (*User, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	id, ok := r.byNIK[nikToken]
	if !ok {
		return nil, ErrNotFound
	}
	u := r.users[id]
	return copyUser(u), nil
}

func (r *InMemoryUserRepository) UpdateVerificationStatus(_ context.Context, id, status string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	u, ok := r.users[id]
	if !ok {
		return ErrNotFound
	}
	u.VerificationStatus = status
	u.UpdatedAt = time.Now().UTC()
	return nil
}

func (r *InMemoryUserRepository) Exists(_ context.Context, nikToken string) (bool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	_, ok := r.byNIK[nikToken]
	return ok, nil
}

func copyUser(u *User) *User {
	cp := *u
	cp.NameEnc = copyBytes(u.NameEnc)
	cp.DOBEnc = copyBytes(u.DOBEnc)
	cp.PhoneEnc = copyBytes(u.PhoneEnc)
	cp.EmailEnc = copyBytes(u.EmailEnc)
	cp.AddressEnc = copyBytes(u.AddressEnc)
	cp.WrappedDEK = copyBytes(u.WrappedDEK)
	if u.DukcapilVerifiedAt != nil {
		t := *u.DukcapilVerifiedAt
		cp.DukcapilVerifiedAt = &t
	}
	return &cp
}

func copyBytes(b []byte) []byte {
	if b == nil {
		return nil
	}
	cp := make([]byte, len(b))
	copy(cp, b)
	return cp
}

func generateUUID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	// Set version 4 and variant bits.
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%s-%s-%s-%s-%s",
		hex.EncodeToString(b[0:4]),
		hex.EncodeToString(b[4:6]),
		hex.EncodeToString(b[6:8]),
		hex.EncodeToString(b[8:10]),
		hex.EncodeToString(b[10:16]),
	), nil
}
