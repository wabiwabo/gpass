package pkce

import (
	"testing"
)

func TestGenerate(t *testing.T) {
	pair, err := Generate()
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	if len(pair.Verifier) != 43 {
		t.Errorf("verifier len = %d, want 43", len(pair.Verifier))
	}
	if pair.Challenge == "" {
		t.Error("challenge should not be empty")
	}
	if pair.Method != MethodS256 {
		t.Errorf("method = %q, want S256", pair.Method)
	}
}

func TestGenerate_UniqueVerifiers(t *testing.T) {
	seen := make(map[string]bool)
	for i := 0; i < 100; i++ {
		pair, _ := Generate()
		if seen[pair.Verifier] {
			t.Fatal("duplicate verifier")
		}
		seen[pair.Verifier] = true
	}
}

func TestGenerateWithLength(t *testing.T) {
	tests := []struct {
		name   string
		length int
		wantOK bool
	}{
		{"min length", 43, true},
		{"max length", 128, true},
		{"medium", 64, true},
		{"too short", 42, false},
		{"too long", 129, false},
		{"zero", 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pair, err := GenerateWithLength(tt.length)
			if tt.wantOK {
				if err != nil {
					t.Fatalf("error: %v", err)
				}
				if len(pair.Verifier) != tt.length {
					t.Errorf("verifier len = %d, want %d", len(pair.Verifier), tt.length)
				}
			} else {
				if err == nil {
					t.Error("expected error")
				}
			}
		})
	}
}

func TestChallengeS256_Deterministic(t *testing.T) {
	verifier := "dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk"
	c1 := ChallengeS256(verifier)
	c2 := ChallengeS256(verifier)
	if c1 != c2 {
		t.Error("S256 challenge should be deterministic")
	}
}

func TestChallengeS256_RFC7636Vector(t *testing.T) {
	// RFC 7636 Appendix B test vector
	verifier := "dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk"
	expected := "E9Melhoa2OwvFrEMTJguCHaoeK1t8URWbuGJSstw-cM"
	challenge := ChallengeS256(verifier)
	if challenge != expected {
		t.Errorf("challenge = %q, want %q (RFC 7636 vector)", challenge, expected)
	}
}

func TestChallengePlain(t *testing.T) {
	verifier := "test-verifier"
	if ChallengePlain(verifier) != verifier {
		t.Error("plain challenge should equal verifier")
	}
}

func TestVerify_S256(t *testing.T) {
	pair, _ := Generate()
	if !Verify(pair.Verifier, pair.Challenge, MethodS256) {
		t.Error("valid S256 pair should verify")
	}
}

func TestVerify_S256_Invalid(t *testing.T) {
	pair, _ := Generate()
	if Verify("wrong-verifier", pair.Challenge, MethodS256) {
		t.Error("wrong verifier should not verify")
	}
}

func TestVerify_Plain(t *testing.T) {
	verifier := "plain-verifier-12345678901234567890123456789012345"
	challenge := ChallengePlain(verifier)
	if !Verify(verifier, challenge, MethodPlain) {
		t.Error("valid plain pair should verify")
	}
}

func TestVerify_Plain_Invalid(t *testing.T) {
	if Verify("wrong", "challenge", MethodPlain) {
		t.Error("wrong verifier should not verify")
	}
}

func TestVerify_UnknownMethod(t *testing.T) {
	if Verify("v", "c", Method("unknown")) {
		t.Error("unknown method should not verify")
	}
}

func TestValidVerifier(t *testing.T) {
	tests := []struct {
		name     string
		verifier string
		want     bool
	}{
		{"valid min length", "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqr", true},
		{"valid with special chars", "ABCDEFGHIJKLMNOPQRSTUVWXYZabcd-._~12345678901", true},
		{"too short", "short", false},
		{"42 chars", "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnop", false},
		{"contains space", "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnop rs", false},
		{"contains invalid char", "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnop+rs", false},
		{"empty", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ValidVerifier(tt.verifier); got != tt.want {
				t.Errorf("ValidVerifier(%q) = %v, want %v", tt.verifier, got, tt.want)
			}
		})
	}
}

func TestValidMethod(t *testing.T) {
	if !ValidMethod(MethodS256) {
		t.Error("S256 should be valid")
	}
	if !ValidMethod(MethodPlain) {
		t.Error("plain should be valid")
	}
	if ValidMethod("unknown") {
		t.Error("unknown should not be valid")
	}
	if ValidMethod("") {
		t.Error("empty should not be valid")
	}
}

func TestGenerate_ValidVerifier(t *testing.T) {
	pair, _ := Generate()
	if !ValidVerifier(pair.Verifier) {
		t.Errorf("generated verifier should be valid: %q", pair.Verifier)
	}
}

func TestGenerateWithLength_ValidVerifier(t *testing.T) {
	pair, _ := GenerateWithLength(128)
	if !ValidVerifier(pair.Verifier) {
		t.Errorf("generated verifier should be valid: %q (len %d)", pair.Verifier, len(pair.Verifier))
	}
}
