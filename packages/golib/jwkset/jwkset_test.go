package jwkset

import (
	"encoding/json"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

func TestNew(t *testing.T) {
	s := New()
	if s.Len() != 0 {
		t.Errorf("Len = %d, want 0", s.Len())
	}
}

func TestAdd_Get(t *testing.T) {
	s := New()
	s.Add(Key{KID: "key-1", KTY: "RSA", Use: "sig", Alg: "RS256"})
	s.Add(Key{KID: "key-2", KTY: "EC", Use: "sig", Alg: "ES256"})

	if s.Len() != 2 {
		t.Errorf("Len = %d, want 2", s.Len())
	}

	k, ok := s.Get("key-1")
	if !ok {
		t.Fatal("key-1 not found")
	}
	if k.KTY != "RSA" {
		t.Errorf("KTY = %q", k.KTY)
	}

	_, ok = s.Get("nonexistent")
	if ok {
		t.Error("should not find nonexistent key")
	}
}

func TestRemove(t *testing.T) {
	s := New()
	s.Add(Key{KID: "key-1"})
	s.Add(Key{KID: "key-2"})

	if !s.Remove("key-1") {
		t.Error("Remove should return true for existing key")
	}
	if s.Len() != 1 {
		t.Errorf("Len = %d, want 1", s.Len())
	}

	if s.Remove("nonexistent") {
		t.Error("Remove should return false for nonexistent key")
	}
}

func TestCurrent(t *testing.T) {
	s := New()
	s.Add(Key{KID: "key-1"})
	s.Add(Key{KID: "key-2"})
	s.Add(Key{KID: "key-3"})

	k, ok := s.Current()
	if !ok {
		t.Fatal("should find current key")
	}
	if k.KID != "key-3" {
		t.Errorf("Current KID = %q, want key-3 (most recent)", k.KID)
	}
}

func TestCurrent_SkipsExpired(t *testing.T) {
	s := New()
	s.Add(Key{KID: "key-1"})
	s.Add(Key{KID: "key-2", ExpiresAt: time.Now().Add(-1 * time.Hour)}) // expired

	k, ok := s.Current()
	if !ok {
		t.Fatal("should find current key")
	}
	if k.KID != "key-1" {
		t.Errorf("Current KID = %q, want key-1 (key-2 is expired)", k.KID)
	}
}

func TestCurrent_AllExpired(t *testing.T) {
	s := New()
	s.Add(Key{KID: "key-1", ExpiresAt: time.Now().Add(-1 * time.Hour)})

	_, ok := s.Current()
	if ok {
		t.Error("should not find current when all expired")
	}
}

func TestCurrent_Empty(t *testing.T) {
	s := New()
	_, ok := s.Current()
	if ok {
		t.Error("should not find current in empty set")
	}
}

func TestKeys_ExcludesExpired(t *testing.T) {
	s := New()
	s.Add(Key{KID: "active-1"})
	s.Add(Key{KID: "expired-1", ExpiresAt: time.Now().Add(-1 * time.Hour)})
	s.Add(Key{KID: "active-2"})

	keys := s.Keys()
	if len(keys) != 2 {
		t.Fatalf("Keys len = %d, want 2", len(keys))
	}
}

func TestAllKeys_IncludesExpired(t *testing.T) {
	s := New()
	s.Add(Key{KID: "active"})
	s.Add(Key{KID: "expired", ExpiresAt: time.Now().Add(-1 * time.Hour)})

	keys := s.AllKeys()
	if len(keys) != 2 {
		t.Fatalf("AllKeys len = %d, want 2", len(keys))
	}
}

func TestPurge(t *testing.T) {
	s := New()
	s.Add(Key{KID: "active"})
	s.Add(Key{KID: "expired-1", ExpiresAt: time.Now().Add(-1 * time.Hour)})
	s.Add(Key{KID: "expired-2", ExpiresAt: time.Now().Add(-2 * time.Hour)})

	removed := s.Purge()
	if removed != 2 {
		t.Errorf("removed = %d, want 2", removed)
	}
	if s.Len() != 1 {
		t.Errorf("Len = %d, want 1", s.Len())
	}
}

func TestPurge_NoneExpired(t *testing.T) {
	s := New()
	s.Add(Key{KID: "key-1"})
	removed := s.Purge()
	if removed != 0 {
		t.Errorf("removed = %d, want 0", removed)
	}
}

func TestKIDs(t *testing.T) {
	s := New()
	s.Add(Key{KID: "c"})
	s.Add(Key{KID: "a"})
	s.Add(Key{KID: "b"})

	kids := s.KIDs()
	want := []string{"a", "b", "c"}
	if len(kids) != 3 {
		t.Fatalf("len = %d", len(kids))
	}
	for i, k := range kids {
		if k != want[i] {
			t.Errorf("KIDs[%d] = %q, want %q", i, k, want[i])
		}
	}
}

func TestIsExpired(t *testing.T) {
	tests := []struct {
		name    string
		key     Key
		expired bool
	}{
		{"no expiry", Key{KID: "a"}, false},
		{"future expiry", Key{KID: "b", ExpiresAt: time.Now().Add(1 * time.Hour)}, false},
		{"past expiry", Key{KID: "c", ExpiresAt: time.Now().Add(-1 * time.Hour)}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.key.IsExpired(); got != tt.expired {
				t.Errorf("IsExpired = %v, want %v", got, tt.expired)
			}
		})
	}
}

func TestHandler(t *testing.T) {
	s := New()
	s.Add(Key{KID: "key-1", KTY: "RSA", Use: "sig", Alg: "RS256"})

	handler := s.Handler()
	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/.well-known/jwks.json", nil)
	handler(w, req)

	if w.Code != 200 {
		t.Errorf("status = %d", w.Code)
	}
	if w.Header().Get("Content-Type") != "application/json" {
		t.Errorf("Content-Type = %q", w.Header().Get("Content-Type"))
	}
	if w.Header().Get("Cache-Control") != "public, max-age=900" {
		t.Errorf("Cache-Control = %q", w.Header().Get("Cache-Control"))
	}

	var resp struct {
		Keys []Key `json:"keys"`
	}
	json.NewDecoder(w.Body).Decode(&resp)
	if len(resp.Keys) != 1 {
		t.Fatalf("keys len = %d", len(resp.Keys))
	}
	if resp.Keys[0].KID != "key-1" {
		t.Errorf("KID = %q", resp.Keys[0].KID)
	}
}

func TestHandler_ExcludesExpired(t *testing.T) {
	s := New()
	s.Add(Key{KID: "active"})
	s.Add(Key{KID: "expired", ExpiresAt: time.Now().Add(-1 * time.Hour)})

	handler := s.Handler()
	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/.well-known/jwks.json", nil)
	handler(w, req)

	var resp struct {
		Keys []Key `json:"keys"`
	}
	json.NewDecoder(w.Body).Decode(&resp)
	if len(resp.Keys) != 1 {
		t.Fatalf("keys len = %d, want 1 (expired excluded)", len(resp.Keys))
	}
}

func TestJSON(t *testing.T) {
	s := New()
	s.Add(Key{KID: "key-1", KTY: "RSA"})

	data, err := s.JSON()
	if err != nil {
		t.Fatalf("JSON: %v", err)
	}

	var resp struct {
		Keys []Key `json:"keys"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(resp.Keys) != 1 {
		t.Errorf("keys len = %d", len(resp.Keys))
	}
}

func TestConcurrent_AddGetRemove(t *testing.T) {
	s := New()
	var wg sync.WaitGroup

	for i := 0; i < 50; i++ {
		wg.Add(3)
		go func() {
			defer wg.Done()
			s.Add(Key{KID: "key"})
		}()
		go func() {
			defer wg.Done()
			s.Get("key")
		}()
		go func() {
			defer wg.Done()
			s.Keys()
		}()
	}
	wg.Wait()
}

func TestAllKeys_CopySlice(t *testing.T) {
	s := New()
	s.Add(Key{KID: "key-1"})
	keys := s.AllKeys()
	s.Add(Key{KID: "key-2"})

	if len(keys) != 1 {
		t.Error("AllKeys should return a copy, not affected by later mutations")
	}
}
