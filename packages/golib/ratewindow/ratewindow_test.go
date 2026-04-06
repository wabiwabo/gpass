package ratewindow

import (
	"sync"
	"testing"
	"time"
)

func TestCounter_Increment(t *testing.T) {
	c := NewCounter(time.Minute)
	v := c.Increment("key")
	if v != 1 {
		t.Errorf("first: got %d", v)
	}
	v = c.Increment("key")
	if v != 2 {
		t.Errorf("second: got %d", v)
	}
}

func TestCounter_Add(t *testing.T) {
	c := NewCounter(time.Minute)
	v := c.Add("key", 5)
	if v != 5 {
		t.Errorf("add 5: got %d", v)
	}
	v = c.Add("key", 3)
	if v != 8 {
		t.Errorf("add 3: got %d", v)
	}
}

func TestCounter_Count(t *testing.T) {
	c := NewCounter(time.Minute)
	c.Increment("key")
	c.Increment("key")

	if cnt := c.Count("key"); cnt != 2 {
		t.Errorf("count: got %d", cnt)
	}
	if cnt := c.Count("missing"); cnt != 0 {
		t.Errorf("missing: got %d", cnt)
	}
}

func TestCounter_WindowExpiry(t *testing.T) {
	c := NewCounter(50 * time.Millisecond)
	c.Increment("key")

	if c.Count("key") != 1 {
		t.Error("should count within window")
	}

	time.Sleep(60 * time.Millisecond)

	if c.Count("key") != 0 {
		t.Error("should return 0 after window expires")
	}
}

func TestCounter_NewWindowAfterExpiry(t *testing.T) {
	c := NewCounter(50 * time.Millisecond)
	c.Increment("key")
	time.Sleep(60 * time.Millisecond)

	v := c.Increment("key")
	if v != 1 {
		t.Errorf("new window should start at 1: got %d", v)
	}
}

func TestCounter_Rate(t *testing.T) {
	c := NewCounter(time.Second)
	for i := 0; i < 100; i++ {
		c.Increment("key")
	}

	rate := c.Rate("key")
	if rate <= 0 {
		t.Error("rate should be positive")
	}
}

func TestCounter_Rate_Missing(t *testing.T) {
	c := NewCounter(time.Second)
	if rate := c.Rate("missing"); rate != 0 {
		t.Errorf("missing rate: got %f", rate)
	}
}

func TestCounter_All(t *testing.T) {
	c := NewCounter(time.Minute)
	c.Increment("a")
	c.Increment("b")
	c.Increment("a")

	all := c.All()
	if all["a"] != 2 {
		t.Errorf("a: got %d", all["a"])
	}
	if all["b"] != 1 {
		t.Errorf("b: got %d", all["b"])
	}
}

func TestCounter_Reset(t *testing.T) {
	c := NewCounter(time.Minute)
	c.Increment("a")
	c.Increment("b")
	c.Reset()

	if c.Size() != 0 {
		t.Errorf("after reset: got %d", c.Size())
	}
}

func TestCounter_Cleanup(t *testing.T) {
	c := NewCounter(20 * time.Millisecond)
	c.Increment("expired")
	time.Sleep(30 * time.Millisecond)
	c.Increment("fresh")

	removed := c.Cleanup()
	if removed != 1 {
		t.Errorf("removed: got %d", removed)
	}
	if c.Size() != 1 {
		t.Errorf("remaining: got %d", c.Size())
	}
}

func TestCounter_Size(t *testing.T) {
	c := NewCounter(time.Minute)
	c.Increment("a")
	c.Increment("b")

	if c.Size() != 2 {
		t.Errorf("size: got %d", c.Size())
	}
}

func TestCounter_DefaultWindow(t *testing.T) {
	c := NewCounter(0) // Should default to 1 minute.
	c.Increment("key")
	if c.Count("key") != 1 {
		t.Error("default window should work")
	}
}

func TestCounter_ConcurrentAccess(t *testing.T) {
	c := NewCounter(time.Minute)
	var wg sync.WaitGroup

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			c.Increment("shared")
		}()
	}
	wg.Wait()

	if cnt := c.Count("shared"); cnt != 100 {
		t.Errorf("concurrent: got %d", cnt)
	}
}
