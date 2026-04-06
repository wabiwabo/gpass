package lazyx

import (
	"errors"
	"sync"
	"sync/atomic"
	"testing"
)

func TestValue_Get(t *testing.T) {
	callCount := 0
	v := New(func() int {
		callCount++
		return 42
	})

	if v.Get() != 42 {
		t.Errorf("Get = %d", v.Get())
	}
	if v.Get() != 42 {
		t.Error("second call should return same value")
	}
	if callCount != 1 {
		t.Errorf("fn called %d times, want 1", callCount)
	}
}

func TestValue_String(t *testing.T) {
	v := New(func() string {
		return "hello"
	})
	if v.Get() != "hello" {
		t.Errorf("Get = %q", v.Get())
	}
}

func TestValue_Concurrent(t *testing.T) {
	var count atomic.Int32
	v := New(func() int {
		count.Add(1)
		return 42
	})

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if v.Get() != 42 {
				t.Error("wrong value")
			}
		}()
	}
	wg.Wait()

	if count.Load() != 1 {
		t.Errorf("fn called %d times, want 1", count.Load())
	}
}

func TestErrorValue_Success(t *testing.T) {
	v := NewWithError(func() (int, error) {
		return 42, nil
	})

	val, err := v.Get()
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if val != 42 {
		t.Errorf("val = %d", val)
	}
}

func TestErrorValue_Failure(t *testing.T) {
	v := NewWithError(func() (int, error) {
		return 0, errors.New("init failed")
	})

	_, err := v.Get()
	if err == nil {
		t.Error("should return error")
	}

	// Second call should return same error
	_, err2 := v.Get()
	if err2 == nil || err2.Error() != "init failed" {
		t.Error("should return cached error")
	}
}

func TestErrorValue_MustGet_Success(t *testing.T) {
	v := NewWithError(func() (string, error) {
		return "ok", nil
	})
	if v.MustGet() != "ok" {
		t.Error("MustGet should return value")
	}
}

func TestErrorValue_MustGet_Panics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("MustGet should panic on error")
		}
	}()

	v := NewWithError(func() (int, error) {
		return 0, errors.New("fail")
	})
	v.MustGet()
}

func TestErrorValue_Concurrent(t *testing.T) {
	var count atomic.Int32
	v := NewWithError(func() (int, error) {
		count.Add(1)
		return 99, nil
	})

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			val, err := v.Get()
			if err != nil || val != 99 {
				t.Error("unexpected result")
			}
		}()
	}
	wg.Wait()

	if count.Load() != 1 {
		t.Errorf("fn called %d times", count.Load())
	}
}

func TestValue_ZeroValue(t *testing.T) {
	v := New(func() int {
		return 0
	})
	if v.Get() != 0 {
		t.Error("should handle zero value")
	}
}
