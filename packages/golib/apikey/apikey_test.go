package apikey

import (
	"strings"
	"testing"
	"time"
)

func TestGenerate(t *testing.T) {
	result, err := Generate("gp_live_", "Production Key")
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	if !strings.HasPrefix(result.Plaintext, "gp_live_") {
		t.Errorf("Plaintext = %q, should start with gp_live_", result.Plaintext)
	}
	if result.Key.Name != "Production Key" {
		t.Errorf("Name = %q", result.Key.Name)
	}
	if result.Key.Prefix != "gp_live_" {
		t.Errorf("Prefix = %q", result.Key.Prefix)
	}
	if result.Key.Hash == "" {
		t.Error("Hash should not be empty")
	}
	if result.Key.ID == "" {
		t.Error("ID should not be empty")
	}
	if !result.Key.Active {
		t.Error("should be active")
	}
	if result.Key.CreatedAt.IsZero() {
		t.Error("CreatedAt should be set")
	}
}

func TestGenerate_DefaultPrefix(t *testing.T) {
	result, _ := Generate("", "test")
	if !strings.HasPrefix(result.Plaintext, "gp_") {
		t.Errorf("default prefix should be gp_, got %q", result.Plaintext)
	}
}

func TestGenerate_PrefixWithoutUnderscore(t *testing.T) {
	result, _ := Generate("myapp", "test")
	if !strings.HasPrefix(result.Plaintext, "myapp_") {
		t.Errorf("should add underscore: %q", result.Plaintext)
	}
}

func TestGenerate_Unique(t *testing.T) {
	seen := make(map[string]bool)
	for i := 0; i < 100; i++ {
		result, _ := Generate("gp_", "test")
		if seen[result.Plaintext] {
			t.Fatal("duplicate key generated")
		}
		seen[result.Plaintext] = true
	}
}

func TestHashKey(t *testing.T) {
	h1 := HashKey("gp_abc123")
	h2 := HashKey("gp_abc123")
	h3 := HashKey("gp_def456")

	if h1 != h2 {
		t.Error("hash should be deterministic")
	}
	if h1 == h3 {
		t.Error("different keys should have different hashes")
	}
	if len(h1) != 64 {
		t.Errorf("hash len = %d, want 64 (SHA-256 hex)", len(h1))
	}
}

func TestVerify_Valid(t *testing.T) {
	result, _ := Generate("gp_", "test")
	if !Verify(result.Plaintext, result.Key.Hash) {
		t.Error("valid key should verify")
	}
}

func TestVerify_Invalid(t *testing.T) {
	result, _ := Generate("gp_", "test")
	if Verify("gp_wrong_key", result.Key.Hash) {
		t.Error("wrong key should not verify")
	}
}

func TestVerify_TamperedHash(t *testing.T) {
	if Verify("gp_anything", "0000000000000000000000000000000000000000000000000000000000000000") {
		t.Error("tampered hash should not verify")
	}
}

func TestKey_IsExpired(t *testing.T) {
	tests := []struct {
		name    string
		exp     time.Time
		expired bool
	}{
		{"not expired", time.Now().Add(1 * time.Hour), false},
		{"expired", time.Now().Add(-1 * time.Hour), true},
		{"no expiry", time.Time{}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			k := Key{ExpiresAt: tt.exp, Active: true}
			if k.IsExpired() != tt.expired {
				t.Errorf("IsExpired = %v", k.IsExpired())
			}
		})
	}
}

func TestKey_IsValid(t *testing.T) {
	tests := []struct {
		name   string
		key    Key
		valid  bool
	}{
		{"active not expired", Key{Active: true}, true},
		{"active expired", Key{Active: true, ExpiresAt: time.Now().Add(-1 * time.Hour)}, false},
		{"inactive", Key{Active: false}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.key.IsValid() != tt.valid {
				t.Errorf("IsValid = %v, want %v", tt.key.IsValid(), tt.valid)
			}
		})
	}
}

func TestKey_HasScope(t *testing.T) {
	k := Key{Scopes: []string{"read", "write"}}

	if !k.HasScope("read") {
		t.Error("should have read")
	}
	if k.HasScope("admin") {
		t.Error("should not have admin")
	}
}

func TestExtractPrefix(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"gp_live_abc123def456", "gp_live_"},
		{"gp_abc123", "gp_"},
		{"sk_test_abc", "sk_test_"},
		{"noprefix", ""},
		{"", ""},
		{"trailing_", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := ExtractPrefix(tt.input); got != tt.want {
				t.Errorf("ExtractPrefix(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestMaskKey(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"gp_live_abc123def456ghi789", "gp_live_abc1..."},
		{"gp_abc123def456", "gp_abc1..."},
		{"gp_ab", "gp_****"},
		{"short", "****"},
		{"ab", "****"},
		{"abcdefghi", "abcd...fghi"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := MaskKey(tt.input)
			if got != tt.want {
				t.Errorf("MaskKey(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestMaskKey_DoesNotLeakFull(t *testing.T) {
	result, _ := Generate("gp_live_", "test")
	masked := MaskKey(result.Plaintext)

	// Masked version should be shorter than original
	if len(masked) >= len(result.Plaintext) {
		t.Error("masked key should be shorter than original")
	}
	// Should not contain the full secret part
	secret := result.Plaintext[len("gp_live_"):]
	if strings.Contains(masked, secret) {
		t.Error("masked key should not contain full secret")
	}
}

func TestGenerate_HashMatchesVerify(t *testing.T) {
	result, _ := Generate("gp_", "test")

	// The stored hash should verify against the plaintext
	if !Verify(result.Plaintext, result.Key.Hash) {
		t.Error("generated hash should verify")
	}

	// And should not verify against a different key
	result2, _ := Generate("gp_", "test2")
	if Verify(result2.Plaintext, result.Key.Hash) {
		t.Error("different key should not verify against original hash")
	}
}
