package idemkey

import (
	"sync"
	"testing"
	"time"
)

func TestNewStore(t *testing.T) {
	s := NewStore(24 * time.Hour)
	if s.Count() != 0 {
		t.Errorf("Count = %d", s.Count())
	}
}

func TestSave_Check(t *testing.T) {
	s := NewStore(1 * time.Hour)
	s.Save("key-1", 200, []byte(`{"id":"123"}`))

	r, ok := s.Check("key-1")
	if !ok {
		t.Fatal("should find result")
	}
	if r.StatusCode != 200 {
		t.Errorf("StatusCode = %d", r.StatusCode)
	}
	if string(r.Body) != `{"id":"123"}` {
		t.Errorf("Body = %q", r.Body)
	}
}

func TestCheck_Missing(t *testing.T) {
	s := NewStore(1 * time.Hour)
	_, ok := s.Check("missing")
	if ok {
		t.Error("should not find missing")
	}
}

func TestCheck_Expired(t *testing.T) {
	s := NewStore(1 * time.Millisecond)
	s.Save("key-1", 200, []byte("data"))

	time.Sleep(5 * time.Millisecond)

	_, ok := s.Check("key-1")
	if ok {
		t.Error("expired should not be found")
	}
}

func TestDelete(t *testing.T) {
	s := NewStore(1 * time.Hour)
	s.Save("key-1", 200, nil)
	s.Delete("key-1")

	_, ok := s.Check("key-1")
	if ok {
		t.Error("should be deleted")
	}
}

func TestPurge(t *testing.T) {
	s := NewStore(1 * time.Millisecond)
	s.Save("a", 200, nil)
	s.Save("b", 200, nil)

	time.Sleep(5 * time.Millisecond)

	removed := s.Purge()
	if removed != 2 {
		t.Errorf("removed = %d", removed)
	}
}

func TestDefaultTTL(t *testing.T) {
	s := NewStore(0)
	if s.ttl != 24*time.Hour {
		t.Errorf("ttl = %v", s.ttl)
	}
}

func TestGenerateKey(t *testing.T) {
	k1 := GenerateKey("POST", "/api/users", []byte(`{"name":"John"}`))
	k2 := GenerateKey("POST", "/api/users", []byte(`{"name":"John"}`))
	k3 := GenerateKey("POST", "/api/users", []byte(`{"name":"Jane"}`))

	if k1 != k2 {
		t.Error("same input should produce same key")
	}
	if k1 == k3 {
		t.Error("different input should produce different key")
	}
	if len(k1) != 64 { // SHA-256 hex
		t.Errorf("len = %d", len(k1))
	}
}

func TestValidateKey(t *testing.T) {
	if err := ValidateKey("valid-key"); err != nil {
		t.Errorf("valid key rejected: %v", err)
	}
	if err := ValidateKey(""); err == nil {
		t.Error("empty should error")
	}

	long := make([]byte, 257)
	for i := range long { long[i] = 'a' }
	if err := ValidateKey(string(long)); err == nil {
		t.Error("too long should error")
	}
}

func TestResult_IsExpired(t *testing.T) {
	r := Result{ExpiresAt: time.Now().Add(-1 * time.Hour)}
	if !r.IsExpired() {
		t.Error("should be expired")
	}

	r2 := Result{ExpiresAt: time.Now().Add(1 * time.Hour)}
	if r2.IsExpired() {
		t.Error("should not be expired")
	}
}

func TestConcurrent(t *testing.T) {
	s := NewStore(1 * time.Hour)
	var wg sync.WaitGroup

	for i := 0; i < 100; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			s.Save("key", 200, []byte("data"))
		}()
		go func() {
			defer wg.Done()
			s.Check("key")
		}()
	}
	wg.Wait()
}
