package nonce

import (
	"sync"
	"testing"
	"time"
)

func TestGenerate(t *testing.T) {
	n, err := Generate(16)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if len(n) != 32 { // 16 bytes = 32 hex chars
		t.Errorf("len = %d, want 32", len(n))
	}
}

func TestGenerate_MinLength(t *testing.T) {
	n, err := Generate(4) // below minimum, should use 16
	if err != nil {
		t.Fatal(err)
	}
	if len(n) != 32 { // defaults to 16 bytes
		t.Errorf("len = %d", len(n))
	}
}

func TestGenerate16(t *testing.T) {
	n, err := Generate16()
	if err != nil {
		t.Fatal(err)
	}
	if len(n) != 32 {
		t.Errorf("len = %d", len(n))
	}
}

func TestGenerate32(t *testing.T) {
	n, err := Generate32()
	if err != nil {
		t.Fatal(err)
	}
	if len(n) != 64 {
		t.Errorf("len = %d", len(n))
	}
}

func TestGenerate_Unique(t *testing.T) {
	seen := make(map[string]bool)
	for i := 0; i < 100; i++ {
		n, _ := Generate16()
		if seen[n] {
			t.Fatal("duplicate nonce")
		}
		seen[n] = true
	}
}

func TestStore_Use(t *testing.T) {
	s := NewStore(5 * time.Minute)
	n, _ := Generate16()

	if !s.Use(n) {
		t.Error("first use should succeed")
	}
	if s.Use(n) {
		t.Error("second use should fail (replay)")
	}
}

func TestStore_IsUsed(t *testing.T) {
	s := NewStore(5 * time.Minute)
	n, _ := Generate16()

	if s.IsUsed(n) {
		t.Error("should not be used yet")
	}
	s.Use(n)
	if !s.IsUsed(n) {
		t.Error("should be used")
	}
}

func TestStore_Count(t *testing.T) {
	s := NewStore(5 * time.Minute)
	s.Use("nonce-1")
	s.Use("nonce-2")
	s.Use("nonce-3")

	if s.Count() != 3 {
		t.Errorf("Count = %d", s.Count())
	}
}

func TestStore_Purge(t *testing.T) {
	s := NewStore(1 * time.Millisecond)
	s.Use("nonce-1")
	s.Use("nonce-2")

	time.Sleep(5 * time.Millisecond)

	removed := s.Purge()
	if removed != 2 {
		t.Errorf("removed = %d", removed)
	}
	if s.Count() != 0 {
		t.Errorf("Count = %d", s.Count())
	}
}

func TestStore_DefaultTTL(t *testing.T) {
	s := NewStore(0)
	if s.ttl != 5*time.Minute {
		t.Errorf("ttl = %v", s.ttl)
	}
}

func TestStore_Concurrent(t *testing.T) {
	s := NewStore(5 * time.Minute)
	var wg sync.WaitGroup

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			nonce, _ := Generate16()
			s.Use(nonce)
		}(i)
	}
	wg.Wait()

	if s.Count() != 100 {
		t.Errorf("Count = %d, want 100", s.Count())
	}
}
