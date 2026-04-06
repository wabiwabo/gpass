package jwtoken

import (
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func makeToken(header, claims map[string]interface{}) string {
	hJSON, _ := json.Marshal(header)
	cJSON, _ := json.Marshal(claims)
	h := base64.RawURLEncoding.EncodeToString(hJSON)
	c := base64.RawURLEncoding.EncodeToString(cJSON)
	return h + "." + c + ".signature"
}

func TestParseHeader(t *testing.T) {
	token := makeToken(
		map[string]interface{}{"alg": "RS256", "typ": "JWT", "kid": "key-1"},
		map[string]interface{}{"sub": "user"},
	)
	h, err := ParseHeader(token)
	if err != nil {
		t.Fatalf("ParseHeader: %v", err)
	}
	if h.Alg != "RS256" { t.Errorf("Alg = %q", h.Alg) }
	if h.Typ != "JWT" { t.Errorf("Typ = %q", h.Typ) }
	if h.Kid != "key-1" { t.Errorf("Kid = %q", h.Kid) }
}

func TestParseClaims(t *testing.T) {
	now := time.Now().Unix()
	token := makeToken(
		map[string]interface{}{"alg": "RS256"},
		map[string]interface{}{
			"iss": "https://auth.garudapass.id",
			"sub": "user-123",
			"aud": "api.garudapass.id",
			"exp": now + 3600,
			"iat": now,
			"jti": "token-id",
		},
	)
	c, err := ParseClaims(token)
	if err != nil {
		t.Fatalf("ParseClaims: %v", err)
	}
	if c.Issuer != "https://auth.garudapass.id" { t.Errorf("Issuer = %q", c.Issuer) }
	if c.Subject != "user-123" { t.Errorf("Subject = %q", c.Subject) }
	if c.JWTID != "token-id" { t.Errorf("JWTID = %q", c.JWTID) }
}

func TestParseClaims_AudienceString(t *testing.T) {
	token := makeToken(
		map[string]interface{}{"alg": "RS256"},
		map[string]interface{}{"aud": "single-audience"},
	)
	c, _ := ParseClaims(token)
	if len(c.Audience) != 1 || c.Audience[0] != "single-audience" {
		t.Errorf("Audience = %v", c.Audience)
	}
}

func TestParseClaims_AudienceArray(t *testing.T) {
	token := makeToken(
		map[string]interface{}{"alg": "RS256"},
		map[string]interface{}{"aud": []string{"aud1", "aud2"}},
	)
	c, _ := ParseClaims(token)
	if len(c.Audience) != 2 {
		t.Errorf("Audience = %v", c.Audience)
	}
}

func TestIsExpired(t *testing.T) {
	c := Claims{ExpiresAt: time.Now().Unix() - 3600}
	if !c.IsExpired() { t.Error("should be expired") }

	c2 := Claims{ExpiresAt: time.Now().Unix() + 3600}
	if c2.IsExpired() { t.Error("should not be expired") }

	c3 := Claims{}
	if c3.IsExpired() { t.Error("no exp should not be expired") }
}

func TestExpiresTime(t *testing.T) {
	now := time.Now().Unix()
	c := Claims{ExpiresAt: now}
	et := c.ExpiresTime()
	if et.Unix() != now { t.Errorf("ExpiresTime = %v", et) }

	c2 := Claims{}
	if !c2.ExpiresTime().IsZero() { t.Error("zero exp should be zero time") }
}

func TestIssuedTime(t *testing.T) {
	c := Claims{IssuedAt: 1700000000}
	if c.IssuedTime().Unix() != 1700000000 { t.Error("wrong issued time") }

	c2 := Claims{}
	if !c2.IssuedTime().IsZero() { t.Error("zero iat") }
}

func TestParseClaimsMap(t *testing.T) {
	token := makeToken(
		map[string]interface{}{"alg": "RS256"},
		map[string]interface{}{"sub": "user", "custom": "value"},
	)
	m, err := ParseClaimsMap(token)
	if err != nil { t.Fatal(err) }
	if m["custom"] != "value" { t.Errorf("custom = %v", m["custom"]) }
}

func TestExtractKID(t *testing.T) {
	token := makeToken(
		map[string]interface{}{"alg": "RS256", "kid": "my-key"},
		map[string]interface{}{},
	)
	if ExtractKID(token) != "my-key" { t.Error("wrong kid") }
	if ExtractKID("invalid") != "" { t.Error("invalid should return empty") }
}

func TestParse_InvalidToken(t *testing.T) {
	invalids := []string{"", "a.b", "not-a-token", "a.b.c.d"}
	for _, tok := range invalids {
		if _, err := ParseHeader(tok); err == nil && !strings.Contains(tok, ".") {
			t.Errorf("should error for %q", tok)
		}
	}
}
