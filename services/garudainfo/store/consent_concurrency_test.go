package store

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestConcurrentCreate(t *testing.T) {
	s := NewInMemoryConsentStore()
	ctx := context.Background()
	var wg sync.WaitGroup
	n := 100

	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			c := &Consent{
				UserID:          "user-concurrent",
				ClientID:        "client-1",
				Fields:          map[string]bool{"name": true},
				DurationSeconds: 3600,
			}
			if err := s.Create(ctx, c); err != nil {
				t.Errorf("Create %d: %v", idx, err)
			}
		}(i)
	}
	wg.Wait()

	list, _ := s.ListByUser(ctx, "user-concurrent")
	if len(list) != n {
		t.Errorf("got %d consents, want %d", len(list), n)
	}
}

func TestConcurrentReadWrite(t *testing.T) {
	s := NewInMemoryConsentStore()
	ctx := context.Background()

	// Pre-populate
	ids := make([]string, 50)
	for i := 0; i < 50; i++ {
		c := &Consent{
			UserID:          "user-rw",
			ClientID:        "client-1",
			Fields:          map[string]bool{"name": true},
			DurationSeconds: 3600,
		}
		_ = s.Create(ctx, c)
		ids[i] = c.ID
	}

	var wg sync.WaitGroup
	// Concurrent reads
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			_, _ = s.GetByID(ctx, ids[idx%50])
		}(i)
	}
	// Concurrent writes
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			c := &Consent{
				UserID:          "user-rw",
				ClientID:        "client-1",
				Fields:          map[string]bool{"email": true},
				DurationSeconds: 3600,
			}
			_ = s.Create(ctx, c)
		}()
	}
	// Concurrent list
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = s.ListByUser(ctx, "user-rw")
		}()
	}
	wg.Wait()
}

func TestConcurrentRevoke(t *testing.T) {
	s := NewInMemoryConsentStore()
	ctx := context.Background()

	c := &Consent{
		UserID:          "user-rev",
		ClientID:        "client-1",
		Fields:          map[string]bool{"name": true},
		DurationSeconds: 3600,
	}
	_ = s.Create(ctx, c)
	id := c.ID

	// Race multiple revokes — only one should succeed, rest get errors
	var wg sync.WaitGroup
	var successCount atomic.Int32
	var revokedCount atomic.Int32
	var notFoundCount atomic.Int32

	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := s.Revoke(ctx, id)
			if err == nil {
				successCount.Add(1)
			} else if err == ErrConsentRevoked {
				revokedCount.Add(1)
			} else if err == ErrConsentNotFound {
				notFoundCount.Add(1)
			}
		}()
	}
	wg.Wait()

	if successCount.Load() != 1 {
		t.Errorf("exactly 1 revoke should succeed, got %d", successCount.Load())
	}
	if revokedCount.Load() != 19 {
		t.Errorf("19 revokes should get ErrConsentRevoked, got %d", revokedCount.Load())
	}
}

func TestConcurrentExpireStale(t *testing.T) {
	s := NewInMemoryConsentStore()
	ctx := context.Background()

	// Create 50 stale consents
	for i := 0; i < 50; i++ {
		c := &Consent{
			UserID:          "user-expire",
			ClientID:        "client-1",
			Fields:          map[string]bool{"name": true},
			DurationSeconds: 1,
		}
		_ = s.Create(ctx, c)
	}

	// Force all to be expired
	s.mu.Lock()
	past := time.Now().UTC().Add(-1 * time.Hour)
	for _, c := range s.consents {
		c.ExpiresAt = past
	}
	s.mu.Unlock()

	// Run ExpireStale concurrently
	var wg sync.WaitGroup
	var totalExpired atomic.Int32
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			count, _ := s.ExpireStale(ctx)
			totalExpired.Add(int32(count))
		}()
	}
	wg.Wait()

	// Total expired across all goroutines should be exactly 50
	if totalExpired.Load() != 50 {
		t.Errorf("total expired = %d, want 50", totalExpired.Load())
	}
}

func TestConcurrentListActiveByUserAndClient(t *testing.T) {
	s := NewInMemoryConsentStore()
	ctx := context.Background()

	// Create consents for different clients
	for i := 0; i < 20; i++ {
		clientID := "client-a"
		if i%2 == 0 {
			clientID = "client-b"
		}
		c := &Consent{
			UserID:          "user-lac",
			ClientID:        clientID,
			Fields:          map[string]bool{"name": true},
			DurationSeconds: 3600,
		}
		_ = s.Create(ctx, c)
	}

	// Concurrent reads
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			list, err := s.ListActiveByUserAndClient(ctx, "user-lac", "client-a")
			if err != nil {
				t.Errorf("error: %v", err)
			}
			if len(list) != 10 {
				t.Errorf("got %d, want 10", len(list))
			}
		}()
	}
	wg.Wait()
}
