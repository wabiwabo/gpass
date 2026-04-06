package atomicx

import (
	"sync"
	"testing"
	"time"
)

func TestValue(t *testing.T) {
	v := NewValue("hello")
	if v.Load() != "hello" {
		t.Errorf("Load = %q", v.Load())
	}
	v.Store("world")
	if v.Load() != "world" {
		t.Errorf("Load = %q", v.Load())
	}
}

func TestValue_Int(t *testing.T) {
	v := NewValue(42)
	if v.Load() != 42 {
		t.Errorf("Load = %d", v.Load())
	}
}

func TestBool(t *testing.T) {
	b := NewBool(false)
	if b.Load() {
		t.Error("should be false")
	}
	b.Store(true)
	if !b.Load() {
		t.Error("should be true")
	}
}

func TestBool_Toggle(t *testing.T) {
	b := NewBool(false)
	result := b.Toggle()
	if !result {
		t.Error("toggle false→true should return true")
	}
	if !b.Load() {
		t.Error("should be true after toggle")
	}

	result = b.Toggle()
	if result {
		t.Error("toggle true→false should return false")
	}
}

func TestBool_CompareAndSwap(t *testing.T) {
	b := NewBool(false)
	if !b.CompareAndSwap(false, true) {
		t.Error("CAS false→true should succeed")
	}
	if b.CompareAndSwap(false, true) {
		t.Error("CAS false→true should fail (already true)")
	}
}

func TestDuration(t *testing.T) {
	d := NewDuration(5 * time.Second)
	if d.Load() != 5*time.Second {
		t.Errorf("Load = %v", d.Load())
	}
	d.Store(10 * time.Second)
	if d.Load() != 10*time.Second {
		t.Errorf("Load = %v", d.Load())
	}
}

func TestString(t *testing.T) {
	s := NewString("hello")
	if s.Load() != "hello" {
		t.Errorf("Load = %q", s.Load())
	}
	s.Store("world")
	if s.Load() != "world" {
		t.Errorf("Load = %q", s.Load())
	}
}

func TestConcurrent_Bool(t *testing.T) {
	b := NewBool(false)
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			b.Toggle()
		}()
	}
	wg.Wait()
	// Even number of toggles → back to original
	if b.Load() {
		t.Error("100 toggles should return to false")
	}
}

func TestConcurrent_Duration(t *testing.T) {
	d := NewDuration(0)
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			d.Store(5 * time.Second)
		}()
		go func() {
			defer wg.Done()
			_ = d.Load()
		}()
	}
	wg.Wait()
}

func TestConcurrent_String(t *testing.T) {
	s := NewString("")
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			s.Store("value")
		}()
		go func() {
			defer wg.Done()
			_ = s.Load()
		}()
	}
	wg.Wait()
}

func TestConcurrent_Value(t *testing.T) {
	v := NewValue(0)
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(2)
		go func(n int) {
			defer wg.Done()
			v.Store(n)
		}(i)
		go func() {
			defer wg.Done()
			_ = v.Load()
		}()
	}
	wg.Wait()
}
