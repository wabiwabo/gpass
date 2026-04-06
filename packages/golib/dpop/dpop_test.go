package dpop

import (
	"testing"
	"time"
)

func TestComputeTokenHash(t *testing.T) {
	// Deterministic
	h1 := ComputeTokenHash("access-token-123")
	h2 := ComputeTokenHash("access-token-123")
	if h1 != h2 {
		t.Error("hash should be deterministic")
	}

	// Different tokens produce different hashes
	h3 := ComputeTokenHash("access-token-456")
	if h1 == h3 {
		t.Error("different tokens should produce different hashes")
	}

	// Not empty
	if h1 == "" {
		t.Error("hash should not be empty")
	}
}

func TestValidateBinding_Valid(t *testing.T) {
	proof := Proof{
		JTI: "unique-id-123",
		HTM: "POST",
		HTU: "https://api.example.com/token",
		IAT: time.Now(),
	}

	if err := ValidateBinding(proof, "POST", "https://api.example.com/token", DefaultConfig()); err != nil {
		t.Errorf("valid proof should pass: %v", err)
	}
}

func TestValidateBinding_MissingJTI(t *testing.T) {
	proof := Proof{
		HTM: "GET",
		HTU: "https://api.example.com/resource",
		IAT: time.Now(),
	}

	err := ValidateBinding(proof, "GET", "https://api.example.com/resource", DefaultConfig())
	if err != ErrMissingJTI {
		t.Errorf("err = %v, want ErrMissingJTI", err)
	}
}

func TestValidateBinding_MethodMismatch(t *testing.T) {
	proof := Proof{
		JTI: "id",
		HTM: "GET",
		HTU: "https://api.example.com/resource",
		IAT: time.Now(),
	}

	err := ValidateBinding(proof, "POST", "https://api.example.com/resource", DefaultConfig())
	if err != ErrMethodMismatch {
		t.Errorf("err = %v, want ErrMethodMismatch", err)
	}
}

func TestValidateBinding_MethodCaseInsensitive(t *testing.T) {
	proof := Proof{
		JTI: "id",
		HTM: "get",
		HTU: "https://api.example.com/resource",
		IAT: time.Now(),
	}

	if err := ValidateBinding(proof, "GET", "https://api.example.com/resource", DefaultConfig()); err != nil {
		t.Errorf("should be case-insensitive: %v", err)
	}
}

func TestValidateBinding_URIMismatch(t *testing.T) {
	proof := Proof{
		JTI: "id",
		HTM: "GET",
		HTU: "https://api.example.com/other",
		IAT: time.Now(),
	}

	err := ValidateBinding(proof, "GET", "https://api.example.com/resource", DefaultConfig())
	if err != ErrURIMismatch {
		t.Errorf("err = %v, want ErrURIMismatch", err)
	}
}

func TestValidateBinding_URINormalization(t *testing.T) {
	tests := []struct {
		name     string
		proofURI string
		expected string
		match    bool
	}{
		{"strip query", "https://api.com/path?q=1", "https://api.com/path", true},
		{"strip fragment", "https://api.com/path#section", "https://api.com/path", true},
		{"strip trailing slash", "https://api.com/path/", "https://api.com/path", true},
		{"exact match", "https://api.com/path", "https://api.com/path", true},
		{"different path", "https://api.com/other", "https://api.com/path", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			proof := Proof{
				JTI: "id",
				HTM: "GET",
				HTU: tt.proofURI,
				IAT: time.Now(),
			}
			err := ValidateBinding(proof, "GET", tt.expected, DefaultConfig())
			if tt.match && err != nil {
				t.Errorf("should match: %v", err)
			}
			if !tt.match && err == nil {
				t.Error("should not match")
			}
		})
	}
}

func TestValidateBinding_Expired(t *testing.T) {
	proof := Proof{
		JTI: "id",
		HTM: "GET",
		HTU: "https://api.example.com/resource",
		IAT: time.Now().Add(-10 * time.Minute),
	}

	err := ValidateBinding(proof, "GET", "https://api.example.com/resource", DefaultConfig())
	if err != ErrExpiredProof {
		t.Errorf("err = %v, want ErrExpiredProof", err)
	}
}

func TestValidateBinding_FutureProof(t *testing.T) {
	proof := Proof{
		JTI: "id",
		HTM: "GET",
		HTU: "https://api.example.com/resource",
		IAT: time.Now().Add(5 * time.Minute), // far future
	}

	err := ValidateBinding(proof, "GET", "https://api.example.com/resource", DefaultConfig())
	if err != ErrExpiredProof {
		t.Errorf("err = %v, want ErrExpiredProof", err)
	}
}

func TestValidateBinding_CustomMaxAge(t *testing.T) {
	proof := Proof{
		JTI: "id",
		HTM: "GET",
		HTU: "https://api.example.com/resource",
		IAT: time.Now().Add(-3 * time.Minute),
	}

	cfg := Config{MaxAge: 1 * time.Minute}
	err := ValidateBinding(proof, "GET", "https://api.example.com/resource", cfg)
	if err != ErrExpiredProof {
		t.Errorf("err = %v, want ErrExpiredProof", err)
	}

	cfg2 := Config{MaxAge: 10 * time.Minute}
	err2 := ValidateBinding(proof, "GET", "https://api.example.com/resource", cfg2)
	if err2 != nil {
		t.Errorf("should pass with larger MaxAge: %v", err2)
	}
}

func TestVerifyTokenBinding_Valid(t *testing.T) {
	token := "my-access-token"
	proof := Proof{ATH: ComputeTokenHash(token)}

	if err := VerifyTokenBinding(proof, token); err != nil {
		t.Errorf("valid binding should pass: %v", err)
	}
}

func TestVerifyTokenBinding_Invalid(t *testing.T) {
	proof := Proof{ATH: "wrong-hash"}
	err := VerifyTokenBinding(proof, "my-access-token")
	if err != ErrTokenHashFailed {
		t.Errorf("err = %v, want ErrTokenHashFailed", err)
	}
}

func TestIsMethodAllowed(t *testing.T) {
	tests := []struct {
		name    string
		method  string
		allowed []string
		want    bool
	}{
		{"no restriction", "GET", nil, true},
		{"empty allowed", "GET", []string{}, true},
		{"in list", "POST", []string{"GET", "POST"}, true},
		{"not in list", "DELETE", []string{"GET", "POST"}, false},
		{"case insensitive", "get", []string{"GET"}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsMethodAllowed(tt.method, tt.allowed); got != tt.want {
				t.Errorf("IsMethodAllowed(%q, %v) = %v, want %v", tt.method, tt.allowed, got, tt.want)
			}
		})
	}
}

func TestValidationError_Error(t *testing.T) {
	err := ErrMissingProof
	s := err.Error()
	if s != "dpop: missing_proof: DPoP proof header is required" {
		t.Errorf("Error() = %q", s)
	}
}

func TestDefaultConfig_Values(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.MaxAge != 5*time.Minute {
		t.Errorf("MaxAge = %v", cfg.MaxAge)
	}
	if cfg.NonceRequired {
		t.Error("NonceRequired should be false by default")
	}
}
