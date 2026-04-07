package jwt

import (
	"crypto/ecdsa"
	"encoding/base64"
	"errors"
	"strings"
	"testing"
	"time"
)

// TestParse_AllDecodeErrors pins each base64 / json decode error branch
// in Parse — header bad b64, header bad json, claims bad b64, claims
// bad json, signature bad b64, plus the wrong-part-count branch.
func TestParse_AllDecodeErrors(t *testing.T) {
	good := func(s string) string { return base64.RawURLEncoding.EncodeToString([]byte(s)) }
	cases := []struct {
		name  string
		token string
		want  string
	}{
		{"header not b64", "!!!." + good(`{"alg":"ES256"}`) + "." + good("sig"), "decode header"},
		{"header not json", good("not json") + "." + good(`{}`) + "." + good("sig"), "unmarshal header"},
		{"claims not b64", good(`{"alg":"ES256"}`) + ".!!!." + good("sig"), "decode claims"},
		{"claims not json", good(`{"alg":"ES256"}`) + "." + good("not json") + "." + good("sig"), "unmarshal claims"},
		{"sig not b64", good(`{"alg":"ES256"}`) + "." + good(`{}`) + ".!!!", "decode signature"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Parse(tc.token)
			if err == nil {
				t.Fatal("expected error")
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("err = %q, want substring %q", err.Error(), tc.want)
			}
		})
	}
	// And the very first guard: not three parts.
	if _, err := Parse("only.two"); !errors.Is(err, ErrInvalidFormat) {
		t.Errorf("two parts: %v", err)
	}
}

// TestVerify_RejectionMatrix pins ErrUnknownKey, ErrInvalidSignature
// (wrong key for kid), ErrTokenExpired, ErrTokenNotYetValid by
// constructing tokens whose claims/header force each branch.
func TestVerify_RejectionMatrix(t *testing.T) {
	priv, err := GenerateKeyPair()
	if err != nil {
		t.Fatal(err)
	}
	signer := NewSigner(priv, "kid-1")
	verifier := NewVerifier(map[string]*ecdsa.PublicKey{"kid-1": &priv.PublicKey})

	// Happy path sanity.
	tok, err := signer.Sign(Claims{Subject: "s"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := verifier.Verify(tok); err != nil {
		t.Fatalf("happy path: %v", err)
	}

	// Unknown kid.
	other := NewSigner(priv, "kid-other")
	tok2, _ := other.Sign(Claims{Subject: "s"})
	if _, err := verifier.Verify(tok2); !errors.Is(err, ErrUnknownKey) {
		t.Errorf("unknown kid: %v", err)
	}

	// Wrong key for kid-1 → invalid signature.
	priv2, _ := GenerateKeyPair()
	v3 := NewVerifier(map[string]*ecdsa.PublicKey{"kid-1": &priv2.PublicKey})
	if _, err := v3.Verify(tok); !errors.Is(err, ErrInvalidSignature) {
		t.Errorf("wrong key: %v", err)
	}

	// nbf in the future.
	future, _ := signer.Sign(Claims{Subject: "s", NotBefore: time.Now().Add(time.Hour)})
	if _, err := verifier.Verify(future); !errors.Is(err, ErrTokenNotYetValid) {
		t.Errorf("nbf future: %v", err)
	}

	// Expired.
	expired, _ := signer.Sign(Claims{Subject: "s", ExpiresAt: time.Now().Add(-time.Hour)})
	if _, err := verifier.Verify(expired); !errors.Is(err, ErrTokenExpired) {
		t.Errorf("expired: %v", err)
	}
}

// TestSign_RoundTrip pins the canonical Sign → Parse → Verify loop
// including the IEEE P1363 64-byte signature encoding.
func TestSign_RoundTrip(t *testing.T) {
	priv, _ := GenerateKeyPair()
	s := NewSigner(priv, "k")
	v := NewVerifier(map[string]*ecdsa.PublicKey{"k": &priv.PublicKey})

	tok, err := s.Sign(Claims{Subject: "alice", Issuer: "iss"})
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := Parse(tok)
	if err != nil {
		t.Fatal(err)
	}
	if len(parsed.Signature) != 64 {
		t.Errorf("signature len = %d, want 64", len(parsed.Signature))
	}
	verified, err := v.Verify(tok)
	if err != nil {
		t.Fatal(err)
	}
	if verified.Claims.Subject != "alice" || verified.Claims.Issuer != "iss" {
		t.Errorf("verified claims: %+v", verified.Claims)
	}
}
