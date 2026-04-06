package par

import (
	"strings"
	"sync"
	"testing"
	"time"
)

func TestNewStore(t *testing.T) {
	s := NewStore(60 * time.Second)
	if s.Count() != 0 {
		t.Errorf("Count = %d", s.Count())
	}
}

func TestNewStore_DefaultTTL(t *testing.T) {
	s := NewStore(0)
	if s.ttl != 60*time.Second {
		t.Errorf("ttl = %v, want 60s", s.ttl)
	}
}

func TestPush(t *testing.T) {
	s := NewStore(60 * time.Second)
	uri, err := s.Push(Request{
		ClientID:     "client-1",
		RedirectURI:  "https://app.example.com/callback",
		ResponseType: "code",
		Scope:        "openid profile",
		State:        "xyz",
	})

	if err != nil {
		t.Fatalf("Push: %v", err)
	}
	if !strings.HasPrefix(uri, "urn:ietf:params:oauth:request_uri:") {
		t.Errorf("uri = %q, want urn:ietf:params:oauth:request_uri: prefix", uri)
	}
	if s.Count() != 1 {
		t.Errorf("Count = %d", s.Count())
	}
}

func TestPush_UniqueURIs(t *testing.T) {
	s := NewStore(60 * time.Second)
	uris := make(map[string]bool)

	for i := 0; i < 100; i++ {
		uri, _ := s.Push(Request{ClientID: "client"})
		if uris[uri] {
			t.Fatal("duplicate URI generated")
		}
		uris[uri] = true
	}
}

func TestConsume_ValidRequest(t *testing.T) {
	s := NewStore(60 * time.Second)
	uri, _ := s.Push(Request{
		ClientID:    "client-1",
		Scope:       "openid",
		RedirectURI: "https://app.example.com/cb",
	})

	req, ok := s.Consume(uri)
	if !ok {
		t.Fatal("should find request")
	}
	if req.ClientID != "client-1" {
		t.Errorf("ClientID = %q", req.ClientID)
	}
	if req.RequestURI != uri {
		t.Errorf("RequestURI = %q", req.RequestURI)
	}
	if req.CreatedAt.IsZero() {
		t.Error("CreatedAt should be set")
	}
	if req.ExpiresAt.IsZero() {
		t.Error("ExpiresAt should be set")
	}
}

func TestConsume_OneTimeUse(t *testing.T) {
	s := NewStore(60 * time.Second)
	uri, _ := s.Push(Request{ClientID: "client-1"})

	// First consume succeeds
	_, ok1 := s.Consume(uri)
	if !ok1 {
		t.Error("first consume should succeed")
	}

	// Second consume fails
	_, ok2 := s.Consume(uri)
	if ok2 {
		t.Error("second consume should fail (one-time use)")
	}

	if s.Count() != 0 {
		t.Errorf("Count = %d, want 0", s.Count())
	}
}

func TestConsume_Expired(t *testing.T) {
	s := NewStore(1 * time.Nanosecond) // expires immediately
	uri, _ := s.Push(Request{ClientID: "client-1"})

	time.Sleep(1 * time.Millisecond)

	_, ok := s.Consume(uri)
	if ok {
		t.Error("should not consume expired request")
	}
}

func TestConsume_NotFound(t *testing.T) {
	s := NewStore(60 * time.Second)
	_, ok := s.Consume("urn:ietf:params:oauth:request_uri:nonexistent")
	if ok {
		t.Error("should not find nonexistent request")
	}
}

func TestGet(t *testing.T) {
	s := NewStore(60 * time.Second)
	uri, _ := s.Push(Request{ClientID: "client-1"})

	// Get does NOT consume
	req, ok := s.Get(uri)
	if !ok {
		t.Fatal("should find request")
	}
	if req.ClientID != "client-1" {
		t.Errorf("ClientID = %q", req.ClientID)
	}

	// Can Get again
	_, ok2 := s.Get(uri)
	if !ok2 {
		t.Error("Get should not consume")
	}
}

func TestGet_Expired(t *testing.T) {
	s := NewStore(1 * time.Nanosecond)
	uri, _ := s.Push(Request{ClientID: "client-1"})

	time.Sleep(1 * time.Millisecond)

	_, ok := s.Get(uri)
	if ok {
		t.Error("should not return expired request")
	}

	// Expired request should be cleaned up
	if s.Count() != 0 {
		t.Errorf("Count = %d, expired should be removed", s.Count())
	}
}

func TestPurge(t *testing.T) {
	s := NewStore(1 * time.Nanosecond)
	for i := 0; i < 5; i++ {
		s.Push(Request{ClientID: "client"})
	}

	time.Sleep(1 * time.Millisecond)

	removed := s.Purge()
	if removed != 5 {
		t.Errorf("removed = %d, want 5", removed)
	}
	if s.Count() != 0 {
		t.Errorf("Count = %d", s.Count())
	}
}

func TestPurge_NoExpired(t *testing.T) {
	s := NewStore(1 * time.Hour)
	s.Push(Request{ClientID: "client"})

	removed := s.Purge()
	if removed != 0 {
		t.Errorf("removed = %d, want 0", removed)
	}
}

func TestRequest_IsExpired(t *testing.T) {
	r := Request{ExpiresAt: time.Now().Add(-1 * time.Hour)}
	if !r.IsExpired() {
		t.Error("should be expired")
	}

	r2 := Request{ExpiresAt: time.Now().Add(1 * time.Hour)}
	if r2.IsExpired() {
		t.Error("should not be expired")
	}
}

func TestRequest_PKCEFields(t *testing.T) {
	s := NewStore(60 * time.Second)
	uri, _ := s.Push(Request{
		ClientID:            "client-1",
		CodeChallenge:       "E9Melhoa2OwvFrEMTJguCHaoeK1t8URWbuGJSstw-cM",
		CodeChallengeMethod: "S256",
	})

	req, _ := s.Consume(uri)
	if req.CodeChallenge != "E9Melhoa2OwvFrEMTJguCHaoeK1t8URWbuGJSstw-cM" {
		t.Errorf("CodeChallenge = %q", req.CodeChallenge)
	}
	if req.CodeChallengeMethod != "S256" {
		t.Errorf("CodeChallengeMethod = %q", req.CodeChallengeMethod)
	}
}

func TestConcurrent_PushConsume(t *testing.T) {
	s := NewStore(60 * time.Second)
	var wg sync.WaitGroup

	for i := 0; i < 50; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			s.Push(Request{ClientID: "client"})
		}()
		go func() {
			defer wg.Done()
			s.Count()
		}()
	}
	wg.Wait()
}
