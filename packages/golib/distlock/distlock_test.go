package distlock

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

func TestMemoryBackend_TryAcquire(t *testing.T) {
	b := NewMemoryBackend()
	lock, err := b.TryAcquire(context.Background(), "resource-1", "owner-a", time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if lock.Key != "resource-1" {
		t.Errorf("key: got %q", lock.Key)
	}
	if lock.Owner != "owner-a" {
		t.Errorf("owner: got %q", lock.Owner)
	}
	if lock.FencingToken <= 0 {
		t.Error("fencing token should be positive")
	}
}

func TestMemoryBackend_TryAcquire_AlreadyLocked(t *testing.T) {
	b := NewMemoryBackend()
	b.TryAcquire(context.Background(), "r1", "owner-a", time.Second)

	_, err := b.TryAcquire(context.Background(), "r1", "owner-b", time.Second)
	if !errors.Is(err, ErrLockNotAcquired) {
		t.Errorf("expected ErrLockNotAcquired, got %v", err)
	}
}

func TestMemoryBackend_TryAcquire_ExpiredLock(t *testing.T) {
	b := NewMemoryBackend()
	b.TryAcquire(context.Background(), "r1", "owner-a", 10*time.Millisecond)

	time.Sleep(20 * time.Millisecond)

	lock, err := b.TryAcquire(context.Background(), "r1", "owner-b", time.Second)
	if err != nil {
		t.Fatalf("should acquire expired lock: %v", err)
	}
	if lock.Owner != "owner-b" {
		t.Errorf("owner: got %q", lock.Owner)
	}
}

func TestMemoryBackend_TryAcquire_SameOwnerReacquire(t *testing.T) {
	b := NewMemoryBackend()
	b.TryAcquire(context.Background(), "r1", "owner-a", time.Second)

	lock, err := b.TryAcquire(context.Background(), "r1", "owner-a", time.Second)
	if err != nil {
		t.Fatal("same owner should be able to reacquire")
	}
	if lock.FencingToken <= 1 {
		t.Error("reacquire should get new fencing token")
	}
}

func TestMemoryBackend_Release(t *testing.T) {
	b := NewMemoryBackend()
	b.TryAcquire(context.Background(), "r1", "owner-a", time.Second)

	err := b.Release(context.Background(), "r1", "owner-a")
	if err != nil {
		t.Fatal(err)
	}

	// Should be acquirable now.
	_, err = b.TryAcquire(context.Background(), "r1", "owner-b", time.Second)
	if err != nil {
		t.Fatal("should acquire after release")
	}
}

func TestMemoryBackend_Release_WrongOwner(t *testing.T) {
	b := NewMemoryBackend()
	b.TryAcquire(context.Background(), "r1", "owner-a", time.Second)

	err := b.Release(context.Background(), "r1", "owner-b")
	if !errors.Is(err, ErrLockNotAcquired) {
		t.Errorf("wrong owner should fail: %v", err)
	}
}

func TestMemoryBackend_Renew(t *testing.T) {
	b := NewMemoryBackend()
	b.TryAcquire(context.Background(), "r1", "owner-a", 50*time.Millisecond)

	err := b.Renew(context.Background(), "r1", "owner-a", time.Second)
	if err != nil {
		t.Fatal(err)
	}

	// Should not expire for a while.
	time.Sleep(60 * time.Millisecond)
	_, err = b.TryAcquire(context.Background(), "r1", "owner-b", time.Second)
	if !errors.Is(err, ErrLockNotAcquired) {
		t.Error("renewed lock should still be held")
	}
}

func TestMutex_TryLock(t *testing.T) {
	b := NewMemoryBackend()
	m := NewMutex(b, "resource", "owner", time.Second)

	lock, err := m.TryLock(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if lock == nil {
		t.Fatal("expected lock")
	}
	if m.FencingToken() <= 0 {
		t.Error("fencing token should be set")
	}
}

func TestMutex_Lock_BlocksUntilAvailable(t *testing.T) {
	b := NewMemoryBackend()
	m1 := NewMutex(b, "r", "owner-1", 100*time.Millisecond)
	m2 := NewMutex(b, "r", "owner-2", time.Second)

	m1.TryLock(context.Background())

	// m2 should block until m1's lock expires.
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	lock, err := m2.Lock(ctx)
	if err != nil {
		t.Fatalf("should acquire after expiry: %v", err)
	}
	if lock.Owner != "owner-2" {
		t.Errorf("owner: got %q", lock.Owner)
	}
}

func TestMutex_Lock_ContextCancellation(t *testing.T) {
	b := NewMemoryBackend()
	m1 := NewMutex(b, "r", "owner-1", time.Minute)
	m2 := NewMutex(b, "r", "owner-2", time.Second)

	m1.TryLock(context.Background())

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	_, err := m2.Lock(ctx)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("expected deadline exceeded, got %v", err)
	}
}

func TestMutex_Unlock(t *testing.T) {
	b := NewMemoryBackend()
	m := NewMutex(b, "r", "owner", time.Second)

	m.TryLock(context.Background())
	err := m.Unlock(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	if m.FencingToken() != 0 {
		t.Error("fencing token should be 0 after unlock")
	}
}

func TestMutex_Renew(t *testing.T) {
	b := NewMemoryBackend()
	m := NewMutex(b, "r", "owner", time.Second)

	m.TryLock(context.Background())
	err := m.Renew(context.Background())
	if err != nil {
		t.Fatal(err)
	}
}

func TestMutex_Renew_NoLock(t *testing.T) {
	b := NewMemoryBackend()
	m := NewMutex(b, "r", "owner", time.Second)

	err := m.Renew(context.Background())
	if !errors.Is(err, ErrLockExpired) {
		t.Errorf("expected ErrLockExpired, got %v", err)
	}
}

func TestFencingGuard_Validate(t *testing.T) {
	g := NewFencingGuard()

	if err := g.Validate(1); err != nil {
		t.Fatal(err)
	}
	if err := g.Validate(2); err != nil {
		t.Fatal(err)
	}
	if err := g.Validate(1); !errors.Is(err, ErrFencingTokenStale) {
		t.Errorf("stale token should fail: %v", err)
	}
}

func TestFencingGuard_LastToken(t *testing.T) {
	g := NewFencingGuard()
	g.Validate(5)
	g.Validate(10)
	if g.LastToken() != 10 {
		t.Errorf("last token: got %d", g.LastToken())
	}
}

func TestFencingToken_MonotonicallyIncreasing(t *testing.T) {
	b := NewMemoryBackend()
	var tokens []int64

	for i := 0; i < 10; i++ {
		lock, _ := b.TryAcquire(context.Background(), "r", "owner", 10*time.Millisecond)
		tokens = append(tokens, lock.FencingToken)
		time.Sleep(15 * time.Millisecond)
	}

	for i := 1; i < len(tokens); i++ {
		if tokens[i] <= tokens[i-1] {
			t.Errorf("token %d (%d) should be > token %d (%d)", i, tokens[i], i-1, tokens[i-1])
		}
	}
}

func TestMutex_ConcurrentLocking(t *testing.T) {
	b := NewMemoryBackend()
	var wg sync.WaitGroup
	acquired := make([]int64, 10)

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			m := NewMutex(b, "shared", "owner-"+string(rune('A'+idx)), 20*time.Millisecond)
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()

			lock, err := m.Lock(ctx)
			if err == nil {
				acquired[idx] = lock.FencingToken
				time.Sleep(10 * time.Millisecond)
				m.Unlock(context.Background())
			}
		}(i)
	}
	wg.Wait()

	// At least one should have acquired.
	hasAcquired := false
	for _, tok := range acquired {
		if tok > 0 {
			hasAcquired = true
			break
		}
	}
	if !hasAcquired {
		t.Error("at least one goroutine should have acquired the lock")
	}
}

func TestLock_IsExpired(t *testing.T) {
	lock := &Lock{ExpiresAt: time.Now().Add(-time.Second)}
	if !lock.IsExpired() {
		t.Error("should be expired")
	}

	lock = &Lock{ExpiresAt: time.Now().Add(time.Hour)}
	if lock.IsExpired() {
		t.Error("should not be expired")
	}
}
