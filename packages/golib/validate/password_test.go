package validate

import (
	"testing"
)

func TestValidatePassword_StrongPassword(t *testing.T) {
	strength, err := ValidatePassword("Gp@ss2026!Secure", DefaultPasswordConfig())
	if err != nil {
		t.Fatalf("strong password should pass: %v", err)
	}
	if strength.Score < 60 {
		t.Errorf("strong password score: got %d, expected >= 60", strength.Score)
	}
	if strength.Level != "strong" && strength.Level != "very_strong" {
		t.Errorf("expected strong/very_strong, got %q", strength.Level)
	}
	if strength.Entropy <= 0 {
		t.Error("entropy should be positive")
	}
}

func TestValidatePassword_TooShort(t *testing.T) {
	_, err := ValidatePassword("Ab1!", DefaultPasswordConfig())
	if err == nil {
		t.Error("short password should fail")
	}
}

func TestValidatePassword_TooLong(t *testing.T) {
	long := ""
	for i := 0; i < 200; i++ {
		long += "A"
	}
	_, err := ValidatePassword(long, DefaultPasswordConfig())
	if err == nil {
		t.Error("long password should fail")
	}
}

func TestValidatePassword_MissingUpper(t *testing.T) {
	_, err := ValidatePassword("password1!", DefaultPasswordConfig())
	if err == nil {
		t.Error("missing uppercase should fail")
	}
}

func TestValidatePassword_MissingLower(t *testing.T) {
	_, err := ValidatePassword("PASSWORD1!", DefaultPasswordConfig())
	if err == nil {
		t.Error("missing lowercase should fail")
	}
}

func TestValidatePassword_MissingDigit(t *testing.T) {
	_, err := ValidatePassword("Password!!", DefaultPasswordConfig())
	if err == nil {
		t.Error("missing digit should fail")
	}
}

func TestValidatePassword_MissingSymbol(t *testing.T) {
	_, err := ValidatePassword("Password12", DefaultPasswordConfig())
	if err == nil {
		t.Error("missing symbol should fail")
	}
}

func TestValidatePassword_CommonPassword(t *testing.T) {
	// Exact common passwords should be rejected (case-insensitive).
	_, err := ValidatePassword("Password", PasswordConfig{MinLength: 1})
	if err == nil {
		t.Error("'Password' (common) should fail")
	}

	_, err = ValidatePassword("QWERTY", PasswordConfig{MinLength: 1})
	if err == nil {
		t.Error("'QWERTY' (common) should fail")
	}
}

func TestValidatePassword_IndonesianCommon(t *testing.T) {
	commonIndo := []string{"rahasia", "bismillah", "pancasila", "merdeka", "garuda"}
	for _, pw := range commonIndo {
		strength, _ := ValidatePassword(pw, PasswordConfig{MinLength: 1})
		if strength.Score > 20 {
			t.Errorf("Indonesian common password %q should score low, got %d", pw, strength.Score)
		}
	}
}

func TestValidatePassword_EmptyPassword(t *testing.T) {
	_, err := ValidatePassword("", DefaultPasswordConfig())
	if err == nil {
		t.Error("empty password should fail")
	}
}

func TestValidatePassword_CustomConfig(t *testing.T) {
	cfg := PasswordConfig{
		MinLength:     6,
		MaxLength:     50,
		RequireUpper:  false,
		RequireLower:  true,
		RequireDigit:  true,
		RequireSymbol: false,
		MinScore:      0,
	}

	_, err := ValidatePassword("simple1", cfg)
	if err != nil {
		t.Errorf("custom config should accept 'simple1': %v", err)
	}
}

func TestAnalyzePassword_Entropy(t *testing.T) {
	// Longer, more diverse passwords should have higher entropy.
	s1 := analyzePassword("aaaa")
	s2 := analyzePassword("aA1!")
	s3 := analyzePassword("aA1!bB2@cC3#dD4$")

	if s2.Entropy <= s1.Entropy {
		t.Errorf("diverse chars should have higher entropy: %f vs %f", s2.Entropy, s1.Entropy)
	}
	if s3.Entropy <= s2.Entropy {
		t.Errorf("longer diverse should have highest entropy: %f vs %f", s3.Entropy, s2.Entropy)
	}
}

func TestAnalyzePassword_RepeatingPenalty(t *testing.T) {
	// Compare passwords with same length and character classes.
	normal := analyzePassword("Abxyzq1!")
	repeating := analyzePassword("Aaaaaq1!")

	// Repeating chars should incur a penalty.
	if repeating.Score > normal.Score {
		t.Errorf("repeating should score equal or lower: %d vs %d", repeating.Score, normal.Score)
	}
}

func TestAnalyzePassword_SequentialPenalty(t *testing.T) {
	normal := analyzePassword("Axzqef1!")
	sequential := analyzePassword("Aabcef1!")

	if sequential.Score >= normal.Score {
		t.Errorf("sequential should score lower: %d vs %d", sequential.Score, normal.Score)
	}
}

func TestScoreToLevel(t *testing.T) {
	tests := []struct {
		score int
		level string
	}{
		{0, "very_weak"},
		{10, "very_weak"},
		{20, "weak"},
		{30, "weak"},
		{40, "fair"},
		{50, "fair"},
		{60, "strong"},
		{70, "strong"},
		{80, "very_strong"},
		{100, "very_strong"},
	}

	for _, tt := range tests {
		got := scoreToLevel(tt.score)
		if got != tt.level {
			t.Errorf("scoreToLevel(%d) = %q, want %q", tt.score, got, tt.level)
		}
	}
}

func TestHasRepeatingChars(t *testing.T) {
	if !hasRepeatingChars("aaab") {
		t.Error("'aaab' has repeating chars")
	}
	if hasRepeatingChars("aabb") {
		t.Error("'aabb' should not have 3+ repeating")
	}
	if hasRepeatingChars("abc") {
		t.Error("'abc' has no repeating chars")
	}
}

func TestHasSequentialChars(t *testing.T) {
	if !hasSequentialChars("abc") {
		t.Error("'abc' is sequential")
	}
	if !hasSequentialChars("123") {
		t.Error("'123' is sequential")
	}
	if hasSequentialChars("ace") {
		t.Error("'ace' is not sequential (step of 2)")
	}
}

func TestIsCommonPassword(t *testing.T) {
	if !isCommonPassword("password") {
		t.Error("'password' should be common")
	}
	if !isCommonPassword("PASSWORD") {
		t.Error("case-insensitive: 'PASSWORD' should be common")
	}
	if isCommonPassword("xK9#mP2$") {
		t.Error("random string should not be common")
	}
}
