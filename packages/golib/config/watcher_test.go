package config

import (
	"sync"
	"testing"
)

func TestNewValue_Get(t *testing.T) {
	v := NewValue(42)
	if got := v.Get(); got != 42 {
		t.Errorf("Get() = %d, want 42", got)
	}
}

func TestValue_Set_Get(t *testing.T) {
	v := NewValue("hello")
	v.Set("world")
	if got := v.Get(); got != "world" {
		t.Errorf("Get() = %q, want %q", got, "world")
	}
}

func TestValue_OnChange(t *testing.T) {
	v := NewValue(10)
	var gotOld, gotNew int
	v.OnChange(func(old, new int) {
		gotOld = old
		gotNew = new
	})

	v.Set(20)

	if gotOld != 10 {
		t.Errorf("OnChange old = %d, want 10", gotOld)
	}
	if gotNew != 20 {
		t.Errorf("OnChange new = %d, want 20", gotNew)
	}
}

func TestValue_MultipleOnChange(t *testing.T) {
	v := NewValue("a")
	called1 := false
	called2 := false

	v.OnChange(func(old, new string) {
		called1 = true
	})
	v.OnChange(func(old, new string) {
		called2 = true
	})

	v.Set("b")

	if !called1 {
		t.Error("first OnChange listener not called")
	}
	if !called2 {
		t.Error("second OnChange listener not called")
	}
}

func TestValue_ConcurrentSetGet(t *testing.T) {
	v := NewValue(0)
	var wg sync.WaitGroup
	const n = 1000

	// Writers.
	wg.Add(n)
	for i := range n {
		go func(val int) {
			defer wg.Done()
			v.Set(val)
		}(i)
	}

	// Readers.
	wg.Add(n)
	for range n {
		go func() {
			defer wg.Done()
			_ = v.Get()
		}()
	}

	wg.Wait()

	// Just verify we can still read a value (no panic/race).
	_ = v.Get()
}

func TestSnapshot_SetGet(t *testing.T) {
	s := NewSnapshot()
	s.Set("key1", "value1")

	v, ok := s.Get("key1")
	if !ok {
		t.Fatal("Get returned false for existing key")
	}
	if v != "value1" {
		t.Errorf("Get = %v, want %q", v, "value1")
	}
}

func TestSnapshot_GetMissing(t *testing.T) {
	s := NewSnapshot()
	_, ok := s.Get("missing")
	if ok {
		t.Error("Get returned true for missing key")
	}
}

func TestSnapshot_GetString(t *testing.T) {
	s := NewSnapshot()
	s.Set("name", "GarudaPass")

	got := s.GetString("name", "default")
	if got != "GarudaPass" {
		t.Errorf("GetString = %q, want %q", got, "GarudaPass")
	}

	got = s.GetString("missing", "default")
	if got != "default" {
		t.Errorf("GetString missing = %q, want %q", got, "default")
	}

	// Wrong type returns default.
	s.Set("num", 42)
	got = s.GetString("num", "default")
	if got != "default" {
		t.Errorf("GetString wrong type = %q, want %q", got, "default")
	}
}

func TestSnapshot_GetInt(t *testing.T) {
	s := NewSnapshot()
	s.Set("port", 8080)

	got := s.GetInt("port", 3000)
	if got != 8080 {
		t.Errorf("GetInt = %d, want 8080", got)
	}

	got = s.GetInt("missing", 3000)
	if got != 3000 {
		t.Errorf("GetInt missing = %d, want 3000", got)
	}

	// Wrong type returns default.
	s.Set("str", "hello")
	got = s.GetInt("str", 3000)
	if got != 3000 {
		t.Errorf("GetInt wrong type = %d, want 3000", got)
	}
}

func TestSnapshot_GetBool(t *testing.T) {
	s := NewSnapshot()
	s.Set("enabled", true)

	got := s.GetBool("enabled", false)
	if got != true {
		t.Errorf("GetBool = %v, want true", got)
	}

	got = s.GetBool("missing", false)
	if got != false {
		t.Errorf("GetBool missing = %v, want false", got)
	}

	// Wrong type returns default.
	s.Set("str", "true")
	got = s.GetBool("str", false)
	if got != false {
		t.Errorf("GetBool wrong type = %v, want false", got)
	}
}
