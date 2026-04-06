package cachekey

import (
	"strings"
	"testing"
)

func TestBuilder_Key(t *testing.T) {
	tests := []struct {
		name   string
		prefix string
		parts  []string
		want   string
	}{
		{"single part", "identity", []string{"user"}, "identity:user"},
		{"two parts", "identity", []string{"user", "abc123"}, "identity:user:abc123"},
		{"three parts", "consent", []string{"user", "scope", "email"}, "consent:user:scope:email"},
		{"prefix only", "service", nil, "service"},
		{"empty prefix", "", []string{"key"}, ":key"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := New(tt.prefix)
			got := b.Key(tt.parts...)
			if got != tt.want {
				t.Errorf("Key(%v) = %q, want %q", tt.parts, got, tt.want)
			}
		})
	}
}

func TestBuilder_Hash(t *testing.T) {
	b := New("identity")

	hash1 := b.Hash("user", "abc123")
	hash2 := b.Hash("user", "abc123")
	hash3 := b.Hash("user", "def456")

	// Deterministic
	if hash1 != hash2 {
		t.Errorf("Hash should be deterministic: %q != %q", hash1, hash2)
	}

	// Different inputs produce different hashes
	if hash1 == hash3 {
		t.Error("different inputs should produce different hashes")
	}

	// Starts with prefix
	if !strings.HasPrefix(hash1, "identity:") {
		t.Errorf("hash should start with prefix, got %q", hash1)
	}

	// Has hex suffix (16 chars for 8 bytes)
	parts := strings.SplitN(hash1, ":", 2)
	if len(parts) != 2 {
		t.Fatalf("expected prefix:hash, got %q", hash1)
	}
	if len(parts[1]) != 16 {
		t.Errorf("hash hex should be 16 chars, got %d: %q", len(parts[1]), parts[1])
	}
}

func TestBuilder_Pattern(t *testing.T) {
	tests := []struct {
		name   string
		prefix string
		parts  []string
		want   string
	}{
		{"user pattern", "identity", []string{"user"}, "identity:user:*"},
		{"session pattern", "session", nil, "session:*"},
		{"multi-part", "consent", []string{"user", "scope"}, "consent:user:scope:*"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := New(tt.prefix)
			got := b.Pattern(tt.parts...)
			if got != tt.want {
				t.Errorf("Pattern(%v) = %q, want %q", tt.parts, got, tt.want)
			}
		})
	}
}

func TestUserKey(t *testing.T) {
	got := UserKey("identity", "user123")
	want := "identity:user:user123"
	if got != want {
		t.Errorf("UserKey = %q, want %q", got, want)
	}
}

func TestSessionKey(t *testing.T) {
	got := SessionKey("sess-abc")
	want := "session:sess-abc"
	if got != want {
		t.Errorf("SessionKey = %q, want %q", got, want)
	}
}

func TestConsentKey(t *testing.T) {
	got := ConsentKey("user123", "email")
	want := "consent:user123:email"
	if got != want {
		t.Errorf("ConsentKey = %q, want %q", got, want)
	}
}

func TestEntityKey(t *testing.T) {
	got := EntityKey("ent-456")
	want := "entity:ent-456"
	if got != want {
		t.Errorf("EntityKey = %q, want %q", got, want)
	}
}

func TestRateLimitKey(t *testing.T) {
	got := RateLimitKey("/api/login", "client-xyz")
	want := "ratelimit:/api/login:client-xyz"
	if got != want {
		t.Errorf("RateLimitKey = %q, want %q", got, want)
	}
}

func TestOTPKey(t *testing.T) {
	got := OTPKey("user123", "sms")
	want := "otp:user123:sms"
	if got != want {
		t.Errorf("OTPKey = %q, want %q", got, want)
	}
}

func TestCertKey(t *testing.T) {
	got := CertKey("user123")
	want := "cert:user123"
	if got != want {
		t.Errorf("CertKey = %q, want %q", got, want)
	}
}

func TestHash_Consistency(t *testing.T) {
	b := New("test")
	// Run multiple times to ensure determinism
	results := make(map[string]bool)
	for i := 0; i < 100; i++ {
		results[b.Hash("key", "value")] = true
	}
	if len(results) != 1 {
		t.Errorf("Hash produced %d unique values, want 1", len(results))
	}
}

func TestKey_NoMutation(t *testing.T) {
	b := New("svc")
	key1 := b.Key("a", "b")
	key2 := b.Key("c", "d")
	if key1 == key2 {
		t.Error("different parts should produce different keys")
	}
	// Re-check first key unchanged
	key1Again := b.Key("a", "b")
	if key1 != key1Again {
		t.Errorf("Key should be stable: %q != %q", key1, key1Again)
	}
}

func TestNew_SeparatorDefault(t *testing.T) {
	b := New("test")
	if b.separator != ":" {
		t.Errorf("separator = %q, want ':'", b.separator)
	}
}
