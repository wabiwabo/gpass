package jwt

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestGenerateKeyPair(t *testing.T) {
	key, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("GenerateKeyPair() error: %v", err)
	}
	if key == nil {
		t.Fatal("GenerateKeyPair() returned nil key")
	}
	if key.Curve != elliptic.P256() {
		t.Errorf("GenerateKeyPair() curve = %v, want P-256", key.Curve.Params().Name)
	}
	if key.PublicKey.X == nil || key.PublicKey.Y == nil {
		t.Error("GenerateKeyPair() public key coordinates are nil")
	}
}

func TestSign_ValidToken(t *testing.T) {
	key, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("GenerateKeyPair() error: %v", err)
	}

	signer := NewSigner(key, "key-1")
	claims := Claims{
		Issuer:    "test-issuer",
		Subject:   "user-123",
		ExpiresAt: time.Now().Add(time.Hour),
		IssuedAt:  time.Now(),
	}

	tokenStr, err := signer.Sign(claims)
	if err != nil {
		t.Fatalf("Sign() error: %v", err)
	}

	parts := strings.Split(tokenStr, ".")
	if len(parts) != 3 {
		t.Errorf("Sign() produced %d parts, want 3", len(parts))
	}

	// Verify each part is valid base64url.
	for i, part := range parts {
		if part == "" {
			t.Errorf("Sign() part %d is empty", i)
		}
		if _, err := base64URLDecode(part); err != nil {
			t.Errorf("Sign() part %d is not valid base64url: %v", i, err)
		}
	}
}

func TestSign_Verify_Roundtrip(t *testing.T) {
	key, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("GenerateKeyPair() error: %v", err)
	}

	signer := NewSigner(key, "key-1")
	claims := Claims{
		Issuer:    "garudapass",
		Subject:   "user-456",
		Audience:  "https://api.garudapass.id",
		ExpiresAt: time.Now().Add(time.Hour),
		IssuedAt:  time.Now(),
		NotBefore: time.Now().Add(-time.Minute),
		ID:        "jti-abc-123",
	}

	tokenStr, err := signer.Sign(claims)
	if err != nil {
		t.Fatalf("Sign() error: %v", err)
	}

	verifier := NewVerifier(map[string]*ecdsa.PublicKey{
		"key-1": &key.PublicKey,
	})

	token, err := verifier.Verify(tokenStr)
	if err != nil {
		t.Fatalf("Verify() error: %v", err)
	}

	if token.Claims.Issuer != "garudapass" {
		t.Errorf("Claims.Issuer = %q, want %q", token.Claims.Issuer, "garudapass")
	}
	if token.Claims.Subject != "user-456" {
		t.Errorf("Claims.Subject = %q, want %q", token.Claims.Subject, "user-456")
	}
	if token.Claims.Audience != "https://api.garudapass.id" {
		t.Errorf("Claims.Audience = %q, want %q", token.Claims.Audience, "https://api.garudapass.id")
	}
	if token.Claims.ID != "jti-abc-123" {
		t.Errorf("Claims.ID = %q, want %q", token.Claims.ID, "jti-abc-123")
	}
	if token.Raw != tokenStr {
		t.Error("Token.Raw does not match original token string")
	}
}

func TestVerify_ExpiredToken(t *testing.T) {
	key, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("GenerateKeyPair() error: %v", err)
	}

	signer := NewSigner(key, "key-1")
	claims := Claims{
		Issuer:    "test",
		ExpiresAt: time.Now().Add(-time.Hour), // Expired 1 hour ago.
		IssuedAt:  time.Now().Add(-2 * time.Hour),
	}

	tokenStr, err := signer.Sign(claims)
	if err != nil {
		t.Fatalf("Sign() error: %v", err)
	}

	verifier := NewVerifier(map[string]*ecdsa.PublicKey{
		"key-1": &key.PublicKey,
	})

	_, err = verifier.Verify(tokenStr)
	if err != ErrTokenExpired {
		t.Errorf("Verify() error = %v, want ErrTokenExpired", err)
	}
}

func TestVerify_NotYetValid(t *testing.T) {
	key, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("GenerateKeyPair() error: %v", err)
	}

	signer := NewSigner(key, "key-1")
	claims := Claims{
		Issuer:    "test",
		ExpiresAt: time.Now().Add(2 * time.Hour),
		NotBefore: time.Now().Add(time.Hour), // Not valid for another hour.
		IssuedAt:  time.Now(),
	}

	tokenStr, err := signer.Sign(claims)
	if err != nil {
		t.Fatalf("Sign() error: %v", err)
	}

	verifier := NewVerifier(map[string]*ecdsa.PublicKey{
		"key-1": &key.PublicKey,
	})

	_, err = verifier.Verify(tokenStr)
	if err != ErrTokenNotYetValid {
		t.Errorf("Verify() error = %v, want ErrTokenNotYetValid", err)
	}
}

func TestVerify_WrongKey(t *testing.T) {
	signingKey, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("GenerateKeyPair() error: %v", err)
	}
	wrongKey, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("GenerateKeyPair() error: %v", err)
	}

	signer := NewSigner(signingKey, "key-1")
	claims := Claims{
		Issuer:    "test",
		ExpiresAt: time.Now().Add(time.Hour),
	}

	tokenStr, err := signer.Sign(claims)
	if err != nil {
		t.Fatalf("Sign() error: %v", err)
	}

	verifier := NewVerifier(map[string]*ecdsa.PublicKey{
		"key-1": &wrongKey.PublicKey,
	})

	_, err = verifier.Verify(tokenStr)
	if err != ErrInvalidSignature {
		t.Errorf("Verify() error = %v, want ErrInvalidSignature", err)
	}
}

func TestVerify_TamperedPayload(t *testing.T) {
	key, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("GenerateKeyPair() error: %v", err)
	}

	signer := NewSigner(key, "key-1")
	claims := Claims{
		Issuer:    "test",
		Subject:   "user-123",
		ExpiresAt: time.Now().Add(time.Hour),
	}

	tokenStr, err := signer.Sign(claims)
	if err != nil {
		t.Fatalf("Sign() error: %v", err)
	}

	// Tamper with the payload by replacing it.
	parts := strings.Split(tokenStr, ".")
	tamperedClaims := Claims{
		Issuer:    "test",
		Subject:   "admin",
		ExpiresAt: time.Now().Add(time.Hour),
	}
	tamperedJSON, _ := json.Marshal(tamperedClaims)
	parts[1] = base64URLEncode(tamperedJSON)
	tamperedToken := strings.Join(parts, ".")

	verifier := NewVerifier(map[string]*ecdsa.PublicKey{
		"key-1": &key.PublicKey,
	})

	_, err = verifier.Verify(tamperedToken)
	if err != ErrInvalidSignature {
		t.Errorf("Verify() error = %v, want ErrInvalidSignature", err)
	}
}

func TestVerify_InvalidFormat(t *testing.T) {
	key, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("GenerateKeyPair() error: %v", err)
	}

	verifier := NewVerifier(map[string]*ecdsa.PublicKey{
		"key-1": &key.PublicKey,
	})

	tests := []struct {
		name  string
		token string
	}{
		{"empty string", ""},
		{"no dots", "abcdef"},
		{"one dot", "abc.def"},
		{"too many dots", "a.b.c.d"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := verifier.Verify(tt.token)
			if err == nil {
				t.Error("Verify() expected error for invalid format")
			}
		})
	}
}

func TestParse_WithoutVerification(t *testing.T) {
	key, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("GenerateKeyPair() error: %v", err)
	}

	signer := NewSigner(key, "key-1")
	claims := Claims{
		Issuer:    "parse-test",
		Subject:   "user-789",
		ExpiresAt: time.Now().Add(-time.Hour), // Expired, but Parse should not care.
	}

	tokenStr, err := signer.Sign(claims)
	if err != nil {
		t.Fatalf("Sign() error: %v", err)
	}

	token, err := Parse(tokenStr)
	if err != nil {
		t.Fatalf("Parse() error: %v", err)
	}

	if token.Claims.Issuer != "parse-test" {
		t.Errorf("Parse() Claims.Issuer = %q, want %q", token.Claims.Issuer, "parse-test")
	}
	if token.Claims.Subject != "user-789" {
		t.Errorf("Parse() Claims.Subject = %q, want %q", token.Claims.Subject, "user-789")
	}
	if token.Header["alg"] != "ES256" {
		t.Errorf("Parse() Header[alg] = %q, want %q", token.Header["alg"], "ES256")
	}
}

func TestClaims_CustomFields(t *testing.T) {
	key, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("GenerateKeyPair() error: %v", err)
	}

	signer := NewSigner(key, "key-1")
	claims := Claims{
		Issuer:    "custom-test",
		Subject:   "user-100",
		ExpiresAt: time.Now().Add(time.Hour),
		IssuedAt:  time.Now(),
		Custom: map[string]interface{}{
			"role":     "admin",
			"tenant":   "garudapass",
			"verified": true,
		},
	}

	tokenStr, err := signer.Sign(claims)
	if err != nil {
		t.Fatalf("Sign() error: %v", err)
	}

	verifier := NewVerifier(map[string]*ecdsa.PublicKey{
		"key-1": &key.PublicKey,
	})

	token, err := verifier.Verify(tokenStr)
	if err != nil {
		t.Fatalf("Verify() error: %v", err)
	}

	if token.Claims.Custom["role"] != "admin" {
		t.Errorf("Custom[role] = %v, want %q", token.Claims.Custom["role"], "admin")
	}
	if token.Claims.Custom["tenant"] != "garudapass" {
		t.Errorf("Custom[tenant] = %v, want %q", token.Claims.Custom["tenant"], "garudapass")
	}
	if token.Claims.Custom["verified"] != true {
		t.Errorf("Custom[verified] = %v, want true", token.Claims.Custom["verified"])
	}
}

func TestVerifier_KeyRotation(t *testing.T) {
	key1, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("GenerateKeyPair() error: %v", err)
	}
	key2, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("GenerateKeyPair() error: %v", err)
	}

	signer1 := NewSigner(key1, "key-v1")
	signer2 := NewSigner(key2, "key-v2")

	claims := Claims{
		Issuer:    "rotation-test",
		ExpiresAt: time.Now().Add(time.Hour),
	}

	token1, err := signer1.Sign(claims)
	if err != nil {
		t.Fatalf("signer1.Sign() error: %v", err)
	}
	token2, err := signer2.Sign(claims)
	if err != nil {
		t.Fatalf("signer2.Sign() error: %v", err)
	}

	// Verifier with both keys.
	verifier := NewVerifier(map[string]*ecdsa.PublicKey{
		"key-v1": &key1.PublicKey,
		"key-v2": &key2.PublicKey,
	})

	if _, err := verifier.Verify(token1); err != nil {
		t.Errorf("Verify(token1) error: %v", err)
	}
	if _, err := verifier.Verify(token2); err != nil {
		t.Errorf("Verify(token2) error: %v", err)
	}

	// Verifier with only key2 should reject token1.
	verifierV2Only := NewVerifier(map[string]*ecdsa.PublicKey{
		"key-v2": &key2.PublicKey,
	})

	_, err = verifierV2Only.Verify(token1)
	if err != ErrUnknownKey {
		t.Errorf("Verify(token1) with v2-only verifier: error = %v, want ErrUnknownKey", err)
	}
}

func TestJWKSHandler(t *testing.T) {
	key, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("GenerateKeyPair() error: %v", err)
	}

	handler := JWKSHandler(map[string]*ecdsa.PublicKey{
		"test-key": &key.PublicKey,
	})

	req := httptest.NewRequest(http.MethodGet, "/.well-known/jwks.json", nil)
	rec := httptest.NewRecorder()

	handler(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("JWKSHandler status = %d, want %d", rec.Code, http.StatusOK)
	}

	contentType := rec.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("Content-Type = %q, want %q", contentType, "application/json")
	}

	var jwks JWKS
	if err := json.Unmarshal(rec.Body.Bytes(), &jwks); err != nil {
		t.Fatalf("unmarshal JWKS response: %v", err)
	}

	if len(jwks.Keys) != 1 {
		t.Fatalf("JWKS keys count = %d, want 1", len(jwks.Keys))
	}

	jwk := jwks.Keys[0]
	if jwk.KTY != "EC" {
		t.Errorf("JWK kty = %q, want %q", jwk.KTY, "EC")
	}
	if jwk.CRV != "P-256" {
		t.Errorf("JWK crv = %q, want %q", jwk.CRV, "P-256")
	}
	if jwk.ALG != "ES256" {
		t.Errorf("JWK alg = %q, want %q", jwk.ALG, "ES256")
	}
	if jwk.KID != "test-key" {
		t.Errorf("JWK kid = %q, want %q", jwk.KID, "test-key")
	}
	if jwk.Use != "sig" {
		t.Errorf("JWK use = %q, want %q", jwk.Use, "sig")
	}
	if jwk.X == "" || jwk.Y == "" {
		t.Error("JWK x or y coordinate is empty")
	}
}

func TestSign_Verify_MultipleTokens(t *testing.T) {
	key, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("GenerateKeyPair() error: %v", err)
	}

	signer := NewSigner(key, "multi-key")
	verifier := NewVerifier(map[string]*ecdsa.PublicKey{
		"multi-key": &key.PublicKey,
	})

	subjects := []string{"user-1", "user-2", "user-3", "user-4", "user-5"}
	tokens := make([]string, len(subjects))

	for i, sub := range subjects {
		claims := Claims{
			Issuer:    "multi-test",
			Subject:   sub,
			ExpiresAt: time.Now().Add(time.Hour),
			IssuedAt:  time.Now(),
			ID:        sub + "-jti",
		}

		tokenStr, err := signer.Sign(claims)
		if err != nil {
			t.Fatalf("Sign(%s) error: %v", sub, err)
		}
		tokens[i] = tokenStr
	}

	// All tokens should be unique.
	seen := make(map[string]bool)
	for _, tok := range tokens {
		if seen[tok] {
			t.Error("duplicate token detected")
		}
		seen[tok] = true
	}

	// All tokens should verify successfully.
	for i, tok := range tokens {
		token, err := verifier.Verify(tok)
		if err != nil {
			t.Errorf("Verify(token[%d]) error: %v", i, err)
			continue
		}
		if token.Claims.Subject != subjects[i] {
			t.Errorf("token[%d] Subject = %q, want %q", i, token.Claims.Subject, subjects[i])
		}
	}
}
