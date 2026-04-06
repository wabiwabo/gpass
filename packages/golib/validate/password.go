package validate

import (
	"fmt"
	"math"
	"strings"
	"unicode"
)

// PasswordStrength represents the calculated strength of a password.
type PasswordStrength struct {
	Score    int      `json:"score"`    // 0-100
	Level    string   `json:"level"`    // "very_weak", "weak", "fair", "strong", "very_strong"
	Entropy  float64  `json:"entropy"`  // bits of entropy
	Feedback []string `json:"feedback"` // improvement suggestions
}

// PasswordConfig configures password validation rules.
type PasswordConfig struct {
	MinLength     int  // Default: 8
	MaxLength     int  // Default: 128
	RequireUpper  bool // Default: true
	RequireLower  bool // Default: true
	RequireDigit  bool // Default: true
	RequireSymbol bool // Default: true
	MinScore      int  // Minimum acceptable score (0-100). Default: 40
}

// DefaultPasswordConfig returns enterprise-standard password configuration.
func DefaultPasswordConfig() PasswordConfig {
	return PasswordConfig{
		MinLength:     8,
		MaxLength:     128,
		RequireUpper:  true,
		RequireLower:  true,
		RequireDigit:  true,
		RequireSymbol: true,
		MinScore:      40,
	}
}

// ValidatePassword validates password against the configuration and returns strength analysis.
func ValidatePassword(password string, cfg PasswordConfig) (PasswordStrength, error) {
	if cfg.MinLength == 0 {
		cfg.MinLength = 8
	}
	if cfg.MaxLength == 0 {
		cfg.MaxLength = 128
	}

	strength := analyzePassword(password)
	var errs Errors

	if len(password) < cfg.MinLength {
		errs.Add(fmt.Errorf("password must be at least %d characters", cfg.MinLength))
	}
	if len(password) > cfg.MaxLength {
		errs.Add(fmt.Errorf("password must not exceed %d characters", cfg.MaxLength))
	}

	hasUpper, hasLower, hasDigit, hasSymbol := characterClasses(password)
	if cfg.RequireUpper && !hasUpper {
		errs.Add(fmt.Errorf("password must contain at least one uppercase letter"))
	}
	if cfg.RequireLower && !hasLower {
		errs.Add(fmt.Errorf("password must contain at least one lowercase letter"))
	}
	if cfg.RequireDigit && !hasDigit {
		errs.Add(fmt.Errorf("password must contain at least one digit"))
	}
	if cfg.RequireSymbol && !hasSymbol {
		errs.Add(fmt.Errorf("password must contain at least one special character"))
	}

	if isCommonPassword(password) {
		errs.Add(fmt.Errorf("password is too common"))
		strength.Score = min(strength.Score, 10)
		strength.Level = "very_weak"
	}

	if cfg.MinScore > 0 && strength.Score < cfg.MinScore {
		errs.Add(fmt.Errorf("password strength score %d is below minimum %d", strength.Score, cfg.MinScore))
	}

	if errs.HasErrors() {
		return strength, &errs
	}

	return strength, nil
}

func analyzePassword(password string) PasswordStrength {
	length := len(password)
	if length == 0 {
		return PasswordStrength{Level: "very_weak"}
	}

	entropy := calculateEntropy(password)
	hasUpper, hasLower, hasDigit, hasSymbol := characterClasses(password)

	score := 0
	var feedback []string

	// Length score (max 30).
	switch {
	case length >= 16:
		score += 30
	case length >= 12:
		score += 25
	case length >= 10:
		score += 20
	case length >= 8:
		score += 15
	default:
		score += 5
		feedback = append(feedback, "Use at least 8 characters")
	}

	// Character variety (max 30).
	classes := 0
	if hasUpper {
		classes++
	}
	if hasLower {
		classes++
	}
	if hasDigit {
		classes++
	}
	if hasSymbol {
		classes++
	}
	score += classes * 7
	if classes < 3 {
		feedback = append(feedback, "Mix uppercase, lowercase, digits, and symbols")
	}

	// Entropy bonus (max 20).
	switch {
	case entropy >= 60:
		score += 20
	case entropy >= 40:
		score += 15
	case entropy >= 28:
		score += 10
	default:
		score += 5
		feedback = append(feedback, "Increase randomness")
	}

	// Repetition penalty.
	if hasRepeatingChars(password) {
		score -= 10
		feedback = append(feedback, "Avoid repeating characters")
	}

	// Sequential penalty.
	if hasSequentialChars(password) {
		score -= 10
		feedback = append(feedback, "Avoid sequential characters like '123' or 'abc'")
	}

	if score > 100 {
		score = 100
	}
	if score < 0 {
		score = 0
	}

	level := scoreToLevel(score)

	return PasswordStrength{
		Score:    score,
		Level:    level,
		Entropy:  math.Round(entropy*100) / 100,
		Feedback: feedback,
	}
}

func characterClasses(s string) (upper, lower, digit, symbol bool) {
	for _, c := range s {
		switch {
		case unicode.IsUpper(c):
			upper = true
		case unicode.IsLower(c):
			lower = true
		case unicode.IsDigit(c):
			digit = true
		default:
			symbol = true
		}
	}
	return
}

func calculateEntropy(password string) float64 {
	charsetSize := 0
	hasUpper, hasLower, hasDigit, hasSymbol := characterClasses(password)
	if hasLower {
		charsetSize += 26
	}
	if hasUpper {
		charsetSize += 26
	}
	if hasDigit {
		charsetSize += 10
	}
	if hasSymbol {
		charsetSize += 33
	}
	if charsetSize == 0 {
		return 0
	}
	return float64(len(password)) * math.Log2(float64(charsetSize))
}

func hasRepeatingChars(s string) bool {
	runes := []rune(s)
	for i := 2; i < len(runes); i++ {
		if runes[i] == runes[i-1] && runes[i] == runes[i-2] {
			return true
		}
	}
	return false
}

func hasSequentialChars(s string) bool {
	runes := []rune(s)
	for i := 2; i < len(runes); i++ {
		if runes[i]-runes[i-1] == 1 && runes[i-1]-runes[i-2] == 1 {
			return true
		}
	}
	return false
}

func scoreToLevel(score int) string {
	switch {
	case score >= 80:
		return "very_strong"
	case score >= 60:
		return "strong"
	case score >= 40:
		return "fair"
	case score >= 20:
		return "weak"
	default:
		return "very_weak"
	}
}

// Common passwords that should always be rejected.
// Includes Indonesian context-specific entries.
var commonPasswords = map[string]bool{
	"password": true, "123456": true, "12345678": true, "qwerty": true,
	"abc123": true, "monkey": true, "master": true, "dragon": true,
	"111111": true, "baseball": true, "letmein": true, "trustno1": true,
	"password1": true, "password123": true, "welcome": true, "admin": true,
	"login": true, "princess": true, "football": true, "shadow": true,
	// Indonesian common passwords
	"indonesia": true, "jakarta": true, "merdeka": true, "garuda": true,
	"pancasila": true, "nusantara": true, "rahasia": true, "bismillah": true,
	"sayang": true, "cinta": true, "sementara": true, "berhasil": true,
}

func isCommonPassword(password string) bool {
	return commonPasswords[strings.ToLower(password)]
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
