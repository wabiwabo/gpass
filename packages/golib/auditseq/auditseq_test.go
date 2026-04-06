package auditseq

import (
	"strings"
	"sync"
	"testing"
)

func TestNew(t *testing.T) {
	g := New("audit")
	if g.Current() != 0 {
		t.Errorf("Current = %d", g.Current())
	}
}

func TestNext(t *testing.T) {
	g := New("audit")
	if g.Next() != 1 {
		t.Error("first should be 1")
	}
	if g.Next() != 2 {
		t.Error("second should be 2")
	}
	if g.Current() != 2 {
		t.Errorf("Current = %d", g.Current())
	}
}

func TestNext_Monotonic(t *testing.T) {
	g := New("test")
	prev := g.Next()
	for i := 0; i < 1000; i++ {
		n := g.Next()
		if n <= prev {
			t.Fatalf("non-monotonic: %d <= %d", n, prev)
		}
		prev = n
	}
}

func TestNextID(t *testing.T) {
	g := New("AUD")
	id := g.NextID()

	if !strings.HasPrefix(id, "AUD-") {
		t.Errorf("should start with prefix: %q", id)
	}
	parts := strings.Split(id, "-")
	if len(parts) != 3 { // prefix-timestamp-seq
		t.Errorf("parts = %d: %q", len(parts), id)
	}
}

func TestNextID_NoPrefix(t *testing.T) {
	g := New("")
	id := g.NextID()
	// Format: timestamp-seq
	parts := strings.Split(id, "-")
	if len(parts) != 2 {
		t.Errorf("no prefix parts = %d: %q", len(parts), id)
	}
}

func TestNextID_Unique(t *testing.T) {
	g := New("AUD")
	seen := make(map[string]bool)
	for i := 0; i < 100; i++ {
		id := g.NextID()
		if seen[id] {
			t.Fatal("duplicate ID")
		}
		seen[id] = true
	}
}

func TestReset(t *testing.T) {
	g := New("test")
	g.Next()
	g.Next()
	g.Reset(100)

	if g.Current() != 100 {
		t.Errorf("Current = %d", g.Current())
	}
	if g.Next() != 101 {
		t.Error("next after reset should be 101")
	}
}

func TestNextPair(t *testing.T) {
	g := New("test")
	pair := g.NextPair()

	if pair.Before != 1 {
		t.Errorf("Before = %d", pair.Before)
	}
	if pair.After != 2 {
		t.Errorf("After = %d", pair.After)
	}
	if pair.After <= pair.Before {
		t.Error("After should be greater than Before")
	}
}

func TestConcurrent(t *testing.T) {
	g := New("test")
	var wg sync.WaitGroup

	results := make([]int64, 1000)
	for i := 0; i < 1000; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			results[idx] = g.Next()
		}(i)
	}
	wg.Wait()

	// All values should be unique
	seen := make(map[int64]bool)
	for _, v := range results {
		if seen[v] {
			t.Fatal("duplicate sequence number")
		}
		seen[v] = true
	}
	if len(seen) != 1000 {
		t.Errorf("unique = %d, want 1000", len(seen))
	}
}
