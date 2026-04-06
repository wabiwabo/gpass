// Package jwtoken provides JWT token structure parsing and claims
// extraction without cryptographic verification. Used for reading
// token metadata (issuer, expiry, subject) from already-verified
// tokens or for routing/logging purposes.
package jwtoken

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// Header represents a JWT header.
type Header struct {
	Alg string `json:"alg"`
	Typ string `json:"typ"`
	Kid string `json:"kid,omitempty"`
}

// Claims represents standard JWT claims.
type Claims struct {
	Issuer    string   `json:"iss,omitempty"`
	Subject   string   `json:"sub,omitempty"`
	Audience  Audience `json:"aud,omitempty"`
	ExpiresAt int64    `json:"exp,omitempty"`
	NotBefore int64    `json:"nbf,omitempty"`
	IssuedAt  int64    `json:"iat,omitempty"`
	JWTID     string   `json:"jti,omitempty"`
}

// Audience handles both string and []string JSON formats.
type Audience []string

func (a *Audience) UnmarshalJSON(data []byte) error {
	var single string
	if err := json.Unmarshal(data, &single); err == nil {
		*a = Audience{single}
		return nil
	}
	var multi []string
	if err := json.Unmarshal(data, &multi); err != nil {
		return err
	}
	*a = Audience(multi)
	return nil
}

// IsExpired checks if the token has expired.
func (c Claims) IsExpired() bool {
	if c.ExpiresAt == 0 {
		return false
	}
	return time.Now().Unix() > c.ExpiresAt
}

// ExpiresTime returns the expiry as time.Time.
func (c Claims) ExpiresTime() time.Time {
	if c.ExpiresAt == 0 {
		return time.Time{}
	}
	return time.Unix(c.ExpiresAt, 0)
}

// IssuedTime returns the issued-at as time.Time.
func (c Claims) IssuedTime() time.Time {
	if c.IssuedAt == 0 {
		return time.Time{}
	}
	return time.Unix(c.IssuedAt, 0)
}

// ParseHeader extracts the header from a JWT without verification.
func ParseHeader(token string) (Header, error) {
	parts := strings.SplitN(token, ".", 3)
	if len(parts) != 3 {
		return Header{}, fmt.Errorf("jwtoken: invalid token format")
	}
	var h Header
	if err := decodeSegment(parts[0], &h); err != nil {
		return Header{}, fmt.Errorf("jwtoken: invalid header: %w", err)
	}
	return h, nil
}

// ParseClaims extracts claims from a JWT without verification.
func ParseClaims(token string) (Claims, error) {
	parts := strings.SplitN(token, ".", 3)
	if len(parts) != 3 {
		return Claims{}, fmt.Errorf("jwtoken: invalid token format")
	}
	var c Claims
	if err := decodeSegment(parts[1], &c); err != nil {
		return Claims{}, fmt.Errorf("jwtoken: invalid claims: %w", err)
	}
	return c, nil
}

// ParseClaimsMap extracts claims as a generic map.
func ParseClaimsMap(token string) (map[string]interface{}, error) {
	parts := strings.SplitN(token, ".", 3)
	if len(parts) != 3 {
		return nil, fmt.Errorf("jwtoken: invalid token format")
	}
	var m map[string]interface{}
	if err := decodeSegment(parts[1], &m); err != nil {
		return nil, fmt.Errorf("jwtoken: invalid claims: %w", err)
	}
	return m, nil
}

// ExtractKID returns the key ID from a token header.
func ExtractKID(token string) string {
	h, err := ParseHeader(token)
	if err != nil {
		return ""
	}
	return h.Kid
}

func decodeSegment(seg string, v interface{}) error {
	data, err := base64.RawURLEncoding.DecodeString(seg)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, v)
}
