package once

import (
	"errors"
	"sync"
	"sync/atomic"
	"testing"
)

func TestResettable(t *testing.T) {
	var count int
	o := &Resettable{}

	o.Do(func() { count++ })
	o.Do(func() { count++ })

	if count != 1 {
		t.Errorf("count = %d, want 1", count)
	}
}

func TestResettable_Reset(t *testing.T) {
	var count int
	o := &Resettable{}

	o.Do(func() { count++ })
	o.Reset()
	o.Do(func() { count++ })

	if count != 2 {
		t.Errorf("count = %d, want 2 (after reset)", count)
	}
}

func TestResettable_Done(t *testing.T) {
	o := &Resettable{}
	if o.Done() {
		t.Error("should not be done")
	}
	o.Do(func() {})
	if !o.Done() {
		t.Error("should be done")
	}
	o.Reset()
	if o.Done() {
		t.Error("should not be done after reset")
	}
}

func TestResettable_Concurrent(t *testing.T) {
	o := &Resettable{}
	var count atomic.Int32
	var wg sync.WaitGroup

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			o.Do(func() { count.Add(1) })
		}()
	}
	wg.Wait()

	if count.Load() != 1 {
		t.Errorf("count = %d, want 1", count.Load())
	}
}

func TestValue(t *testing.T) {
	callCount := 0
	v := NewValue(func() int {
		callCount++
		return 42
	})

	if v.Get() != 42 {
		t.Error("wrong value")
	}
	if v.Get() != 42 {
		t.Error("second call wrong")
	}
	if callCount != 1 {
		t.Errorf("called %d times", callCount)
	}
}

func TestErrorValue_Success(t *testing.T) {
	v := NewErrorValue(func() (string, error) {
		return "hello", nil
	})
	val, err := v.Get()
	if err != nil || val != "hello" {
		t.Errorf("Get = (%q, %v)", val, err)
	}
}

func TestErrorValue_Error(t *testing.T) {
	v := NewErrorValue(func() (int, error) {
		return 0, errors.New("fail")
	})
	_, err := v.Get()
	if err == nil {
		t.Error("should return error")
	}
	// Cached error
	_, err2 := v.Get()
	if err2 == nil {
		t.Error("should return cached error")
	}
}

func TestRetryOnError_Success(t *testing.T) {
	v := NewRetryOnError(func() (int, error) {
		return 42, nil
	})
	val, err := v.Get()
	if err != nil || val != 42 {
		t.Errorf("Get = (%d, %v)", val, err)
	}
}

func TestRetryOnError_EventualSuccess(t *testing.T) {
	attempt := 0
	v := NewRetryOnError(func() (string, error) {
		attempt++
		if attempt < 3 {
			return "", errors.New("not yet")
		}
		return "success", nil
	})

	// First two attempts fail
	_, err := v.Get()
	if err == nil {
		t.Error("1st should fail")
	}
	_, err = v.Get()
	if err == nil {
		t.Error("2nd should fail")
	}

	// Third succeeds
	val, err := v.Get()
	if err != nil || val != "success" {
		t.Errorf("3rd = (%q, %v)", val, err)
	}

	// Cached from now on
	val, err = v.Get()
	if err != nil || val != "success" {
		t.Error("should be cached")
	}
	if attempt != 3 {
		t.Errorf("attempts = %d, want 3", attempt)
	}
}

func TestValue_Concurrent(t *testing.T) {
	var count atomic.Int32
	v := NewValue(func() int {
		count.Add(1)
		return 99
	})

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			v.Get()
		}()
	}
	wg.Wait()

	if count.Load() != 1 {
		t.Errorf("called %d times", count.Load())
	}
}
