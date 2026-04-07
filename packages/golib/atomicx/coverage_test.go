package atomicx

import (
	"sync"
	"testing"
	"time"
)

// TestValue_NilZeroFallback covers the Load nil-branch: a freshly
// constructed Value[T] (without going through NewValue) returns the
// zero value rather than panicking on a nil type assertion.
func TestValue_NilZeroFallback(t *testing.T) {
	var v Value[int]
	if got := v.Load(); got != 0 {
		t.Errorf("zero Value[int].Load() = %d, want 0", got)
	}
	var s Value[string]
	if got := s.Load(); got != "" {
		t.Errorf("zero Value[string].Load() = %q, want \"\"", got)
	}
}

// TestNewBool_BothInitialValues covers the initial=false branch of
// NewBool which the existing tests skipped.
func TestNewBool_BothInitialValues(t *testing.T) {
	if NewBool(true).Load() != true {
		t.Error("NewBool(true).Load() = false")
	}
	if NewBool(false).Load() != false {
		t.Error("NewBool(false).Load() = true")
	}
}

// TestBool_StoreFalseAfterTrue covers the false branch of Store.
func TestBool_StoreFalseAfterTrue(t *testing.T) {
	b := NewBool(true)
	b.Store(false)
	if b.Load() {
		t.Error("after Store(false), Load() = true")
	}
	b.Store(true)
	if !b.Load() {
		t.Error("after Store(true), Load() = false")
	}
}

// TestBool_CompareAndSwapAllCombos pins the four CAS branches: true→true,
// true→false, false→true, false→false. Each must succeed when the
// observed value matches and fail otherwise.
func TestBool_CompareAndSwapAllCombos(t *testing.T) {
	b := NewBool(false)

	// false→false succeeds (no-op CAS)
	if !b.CompareAndSwap(false, false) {
		t.Error("CAS(false, false) on false should succeed")
	}
	// false→true succeeds
	if !b.CompareAndSwap(false, true) {
		t.Error("CAS(false, true) on false should succeed")
	}
	// false→true fails (now true)
	if b.CompareAndSwap(false, true) {
		t.Error("CAS(false, true) on true should fail")
	}
	// true→false succeeds
	if !b.CompareAndSwap(true, false) {
		t.Error("CAS(true, false) on true should succeed")
	}
}

// TestBool_ToggleConcurrent pins that Toggle is race-free under
// contention. Two goroutines toggling 1000 times must converge to the
// initial value (since both do an even number of toggles).
func TestBool_ToggleConcurrent(t *testing.T) {
	b := NewBool(false)
	var wg sync.WaitGroup
	wg.Add(2)
	for g := 0; g < 2; g++ {
		go func() {
			defer wg.Done()
			for i := 0; i < 1000; i++ {
				b.Toggle()
			}
		}()
	}
	wg.Wait()
	if b.Load() != false {
		t.Errorf("after 2000 toggles from false, Load() = %v", b.Load())
	}
}

// TestString_NilZeroFallback covers the Load nil-branch on String.
func TestString_NilZeroFallback(t *testing.T) {
	var s String
	if got := s.Load(); got != "" {
		t.Errorf("zero String.Load() = %q, want \"\"", got)
	}
	s.Store("hello")
	if got := s.Load(); got != "hello" {
		t.Errorf("after Store: Load() = %q", got)
	}
}

// TestDuration_RoundTrip covers Duration's Store/Load round-trip
// (the existing tests didn't touch the new-with-zero path).
func TestDuration_RoundTrip(t *testing.T) {
	d := NewDuration(0)
	if d.Load() != 0 {
		t.Errorf("NewDuration(0).Load() = %v", d.Load())
	}
	d.Store(5 * time.Second)
	if d.Load() != 5*time.Second {
		t.Errorf("after Store(5s): Load() = %v", d.Load())
	}
}
