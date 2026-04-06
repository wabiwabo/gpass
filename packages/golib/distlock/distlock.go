package distlock

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"time"
)

var (
	// ErrLockNotAcquired is returned when the lock cannot be obtained.
	ErrLockNotAcquired = errors.New("distlock: lock not acquired")
	// ErrLockExpired is returned when operating on an expired lock.
	ErrLockExpired = errors.New("distlock: lock expired")
	// ErrFencingTokenStale is returned when a fencing token is outdated.
	ErrFencingTokenStale = errors.New("distlock: stale fencing token")
)

// Lock represents an acquired distributed lock.
type Lock struct {
	Key          string
	FencingToken int64
	Owner        string
	ExpiresAt    time.Time
}

// IsExpired returns true if the lock has expired.
func (l *Lock) IsExpired() bool {
	return time.Now().After(l.ExpiresAt)
}

// Backend is the interface for lock storage (Redis, PostgreSQL, in-memory).
type Backend interface {
	// TryAcquire attempts to acquire a lock. Returns the lock if successful.
	TryAcquire(ctx context.Context, key, owner string, ttl time.Duration) (*Lock, error)
	// Release releases a lock. Only succeeds if the owner matches.
	Release(ctx context.Context, key, owner string) error
	// Renew extends a lock's TTL. Only succeeds if the owner matches.
	Renew(ctx context.Context, key, owner string, ttl time.Duration) error
}

// Mutex provides a high-level distributed mutex API.
type Mutex struct {
	backend Backend
	key     string
	owner   string
	ttl     time.Duration
	lock    *Lock
	mu      sync.Mutex
}

// NewMutex creates a new distributed mutex.
func NewMutex(backend Backend, key, owner string, ttl time.Duration) *Mutex {
	if ttl <= 0 {
		ttl = 30 * time.Second
	}
	return &Mutex{
		backend: backend,
		key:     key,
		owner:   owner,
		ttl:     ttl,
	}
}

// Lock acquires the distributed lock. Blocks until acquired or context cancelled.
func (m *Mutex) Lock(ctx context.Context) (*Lock, error) {
	for {
		lock, err := m.backend.TryAcquire(ctx, m.key, m.owner, m.ttl)
		if err == nil {
			m.mu.Lock()
			m.lock = lock
			m.mu.Unlock()
			return lock, nil
		}

		if !errors.Is(err, ErrLockNotAcquired) {
			return nil, err
		}

		// Wait before retrying.
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(100 * time.Millisecond):
		}
	}
}

// TryLock attempts to acquire the lock without blocking.
func (m *Mutex) TryLock(ctx context.Context) (*Lock, error) {
	lock, err := m.backend.TryAcquire(ctx, m.key, m.owner, m.ttl)
	if err != nil {
		return nil, err
	}
	m.mu.Lock()
	m.lock = lock
	m.mu.Unlock()
	return lock, nil
}

// Unlock releases the distributed lock.
func (m *Mutex) Unlock(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.lock == nil {
		return nil
	}
	err := m.backend.Release(ctx, m.key, m.owner)
	m.lock = nil
	return err
}

// Renew extends the lock's TTL.
func (m *Mutex) Renew(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.lock == nil {
		return ErrLockExpired
	}
	return m.backend.Renew(ctx, m.key, m.owner, m.ttl)
}

// FencingToken returns the current lock's fencing token, or 0 if not held.
func (m *Mutex) FencingToken() int64 {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.lock == nil {
		return 0
	}
	return m.lock.FencingToken
}

// MemoryBackend is an in-memory lock backend for testing.
type MemoryBackend struct {
	mu       sync.Mutex
	locks    map[string]*Lock
	tokenSeq atomic.Int64
}

// NewMemoryBackend creates a new in-memory backend.
func NewMemoryBackend() *MemoryBackend {
	return &MemoryBackend{
		locks: make(map[string]*Lock),
	}
}

// TryAcquire attempts to acquire a lock.
func (b *MemoryBackend) TryAcquire(_ context.Context, key, owner string, ttl time.Duration) (*Lock, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if existing, ok := b.locks[key]; ok {
		if !existing.IsExpired() && existing.Owner != owner {
			return nil, ErrLockNotAcquired
		}
	}

	token := b.tokenSeq.Add(1)
	lock := &Lock{
		Key:          key,
		FencingToken: token,
		Owner:        owner,
		ExpiresAt:    time.Now().Add(ttl),
	}
	b.locks[key] = lock
	return lock, nil
}

// Release releases a lock.
func (b *MemoryBackend) Release(_ context.Context, key, owner string) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if existing, ok := b.locks[key]; ok {
		if existing.Owner == owner {
			delete(b.locks, key)
			return nil
		}
		return ErrLockNotAcquired
	}
	return nil
}

// Renew extends a lock's TTL.
func (b *MemoryBackend) Renew(_ context.Context, key, owner string, ttl time.Duration) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if existing, ok := b.locks[key]; ok {
		if existing.Owner == owner {
			existing.ExpiresAt = time.Now().Add(ttl)
			return nil
		}
		return ErrLockNotAcquired
	}
	return ErrLockExpired
}

// FencingGuard validates that a fencing token is not stale.
type FencingGuard struct {
	mu          sync.Mutex
	lastToken   int64
}

// NewFencingGuard creates a new fencing token validator.
func NewFencingGuard() *FencingGuard {
	return &FencingGuard{}
}

// Validate checks that the token is newer than the last accepted token.
func (g *FencingGuard) Validate(token int64) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	if token <= g.lastToken {
		return ErrFencingTokenStale
	}
	g.lastToken = token
	return nil
}

// LastToken returns the most recently accepted token.
func (g *FencingGuard) LastToken() int64 {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.lastToken
}
