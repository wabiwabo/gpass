package jwt

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"strings"
	"time"
)

// Claims represents the set of claims in a JWT.
type Claims struct {
	Issuer    string                 `json:"iss,omitempty"`
	Subject   string                 `json:"sub,omitempty"`
	Audience  string                 `json:"aud,omitempty"`
	ExpiresAt time.Time              `json:"-"`
	IssuedAt  time.Time              `json:"-"`
	NotBefore time.Time              `json:"-"`
	ID        string                 `json:"jti,omitempty"`
	Custom    map[string]interface{} `json:"-"`
}

// claimsJSON is the JSON-serializable representation of Claims.
type claimsJSON struct {
	Issuer    string `json:"iss,omitempty"`
	Subject   string `json:"sub,omitempty"`
	Audience  string `json:"aud,omitempty"`
	ExpiresAt *int64 `json:"exp,omitempty"`
	IssuedAt  *int64 `json:"iat,omitempty"`
	NotBefore *int64 `json:"nbf,omitempty"`
	ID        string `json:"jti,omitempty"`
}

// MarshalJSON implements custom JSON marshaling for Claims.
func (c Claims) MarshalJSON() ([]byte, error) {
	cj := claimsJSON{
		Issuer:   c.Issuer,
		Subject:  c.Subject,
		Audience: c.Audience,
		ID:       c.ID,
	}
	if !c.ExpiresAt.IsZero() {
		exp := c.ExpiresAt.Unix()
		cj.ExpiresAt = &exp
	}
	if !c.IssuedAt.IsZero() {
		iat := c.IssuedAt.Unix()
		cj.IssuedAt = &iat
	}
	if !c.NotBefore.IsZero() {
		nbf := c.NotBefore.Unix()
		cj.NotBefore = &nbf
	}

	// Marshal standard claims first.
	stdBytes, err := json.Marshal(cj)
	if err != nil {
		return nil, fmt.Errorf("jwt: marshal standard claims: %w", err)
	}

	if len(c.Custom) == 0 {
		return stdBytes, nil
	}

	// Merge custom fields into the standard claims object.
	var merged map[string]interface{}
	if err := json.Unmarshal(stdBytes, &merged); err != nil {
		return nil, fmt.Errorf("jwt: unmarshal for merge: %w", err)
	}
	for k, v := range c.Custom {
		// Do not allow custom fields to override registered claims.
		if _, exists := merged[k]; !exists {
			merged[k] = v
		}
	}
	return json.Marshal(merged)
}

// UnmarshalJSON implements custom JSON unmarshaling for Claims.
func (c *Claims) UnmarshalJSON(data []byte) error {
	var cj claimsJSON
	if err := json.Unmarshal(data, &cj); err != nil {
		return fmt.Errorf("jwt: unmarshal claims: %w", err)
	}
	c.Issuer = cj.Issuer
	c.Subject = cj.Subject
	c.Audience = cj.Audience
	c.ID = cj.ID
	if cj.ExpiresAt != nil {
		c.ExpiresAt = time.Unix(*cj.ExpiresAt, 0)
	}
	if cj.IssuedAt != nil {
		c.IssuedAt = time.Unix(*cj.IssuedAt, 0)
	}
	if cj.NotBefore != nil {
		c.NotBefore = time.Unix(*cj.NotBefore, 0)
	}

	// Extract custom fields.
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("jwt: unmarshal raw claims: %w", err)
	}
	registered := map[string]bool{
		"iss": true, "sub": true, "aud": true,
		"exp": true, "iat": true, "nbf": true, "jti": true,
	}
	c.Custom = make(map[string]interface{})
	for k, v := range raw {
		if registered[k] {
			continue
		}
		var val interface{}
		if err := json.Unmarshal(v, &val); err != nil {
			return fmt.Errorf("jwt: unmarshal custom claim %q: %w", k, err)
		}
		c.Custom[k] = val
	}
	if len(c.Custom) == 0 {
		c.Custom = nil
	}
	return nil
}

// Token represents a parsed JWT.
type Token struct {
	Header    map[string]string
	Claims    Claims
	Signature []byte
	Raw       string
}

// Signer creates signed JWTs using an ECDSA P-256 private key.
type Signer struct {
	key   *ecdsa.PrivateKey
	keyID string
}

// NewSigner creates a new Signer with the given private key and key ID.
func NewSigner(key *ecdsa.PrivateKey, keyID string) *Signer {
	return &Signer{key: key, keyID: keyID}
}

// Sign creates a signed JWT string from the given claims using ES256.
func (s *Signer) Sign(claims Claims) (string, error) {
	header := map[string]string{
		"alg": "ES256",
		"typ": "JWT",
		"kid": s.keyID,
	}

	headerJSON, err := json.Marshal(header)
	if err != nil {
		return "", fmt.Errorf("jwt: marshal header: %w", err)
	}

	claimsJSON, err := json.Marshal(claims)
	if err != nil {
		return "", fmt.Errorf("jwt: marshal claims: %w", err)
	}

	headerB64 := base64URLEncode(headerJSON)
	claimsB64 := base64URLEncode(claimsJSON)
	signingInput := headerB64 + "." + claimsB64

	hash := sha256.Sum256([]byte(signingInput))
	r, ss, err := ecdsa.Sign(rand.Reader, s.key, hash[:])
	if err != nil {
		return "", fmt.Errorf("jwt: sign: %w", err)
	}

	// Encode r and s as fixed-size 32-byte big-endian integers (IEEE P1363 format).
	sigBytes := make([]byte, 64)
	rBytes := r.Bytes()
	sBytes := ss.Bytes()
	copy(sigBytes[32-len(rBytes):32], rBytes)
	copy(sigBytes[64-len(sBytes):64], sBytes)

	sigB64 := base64URLEncode(sigBytes)
	return signingInput + "." + sigB64, nil
}

// Verifier validates JWT signatures using one or more ECDSA P-256 public keys,
// supporting key rotation.
type Verifier struct {
	keys map[string]*ecdsa.PublicKey
}

// NewVerifier creates a new Verifier with the given key ID to public key mapping.
func NewVerifier(keys map[string]*ecdsa.PublicKey) *Verifier {
	return &Verifier{keys: keys}
}

var (
	// ErrInvalidFormat is returned when the JWT string is malformed.
	ErrInvalidFormat = errors.New("jwt: invalid token format")
	// ErrUnsupportedAlgorithm is returned for non-ES256 tokens.
	ErrUnsupportedAlgorithm = errors.New("jwt: unsupported algorithm")
	// ErrUnknownKey is returned when the kid does not match any known key.
	ErrUnknownKey = errors.New("jwt: unknown key ID")
	// ErrInvalidSignature is returned when signature verification fails.
	ErrInvalidSignature = errors.New("jwt: invalid signature")
	// ErrTokenExpired is returned when the token has expired.
	ErrTokenExpired = errors.New("jwt: token expired")
	// ErrTokenNotYetValid is returned when the token's nbf is in the future.
	ErrTokenNotYetValid = errors.New("jwt: token not yet valid")
)

// Verify parses and validates a JWT string, checking the signature, expiry, and nbf.
func (v *Verifier) Verify(tokenStr string) (*Token, error) {
	token, err := Parse(tokenStr)
	if err != nil {
		return nil, err
	}

	alg, ok := token.Header["alg"]
	if !ok || alg != "ES256" {
		return nil, ErrUnsupportedAlgorithm
	}

	kid := token.Header["kid"]
	key, ok := v.keys[kid]
	if !ok {
		return nil, ErrUnknownKey
	}

	// Verify signature.
	parts := strings.SplitN(tokenStr, ".", 3)
	signingInput := parts[0] + "." + parts[1]
	hash := sha256.Sum256([]byte(signingInput))

	if len(token.Signature) != 64 {
		return nil, ErrInvalidSignature
	}

	r := new(big.Int).SetBytes(token.Signature[:32])
	s := new(big.Int).SetBytes(token.Signature[32:64])

	if !ecdsa.Verify(key, hash[:], r, s) {
		return nil, ErrInvalidSignature
	}

	// Check expiry.
	now := time.Now()
	if !token.Claims.ExpiresAt.IsZero() && now.After(token.Claims.ExpiresAt) {
		return nil, ErrTokenExpired
	}

	// Check not-before.
	if !token.Claims.NotBefore.IsZero() && now.Before(token.Claims.NotBefore) {
		return nil, ErrTokenNotYetValid
	}

	return token, nil
}

// Parse decodes a JWT string without verifying the signature.
// Useful for debugging and inspecting token contents.
func Parse(tokenStr string) (*Token, error) {
	parts := strings.SplitN(tokenStr, ".", 3)
	if len(parts) != 3 {
		return nil, ErrInvalidFormat
	}

	headerBytes, err := base64URLDecode(parts[0])
	if err != nil {
		return nil, fmt.Errorf("jwt: decode header: %w", err)
	}

	var header map[string]string
	if err := json.Unmarshal(headerBytes, &header); err != nil {
		return nil, fmt.Errorf("jwt: unmarshal header: %w", err)
	}

	claimsBytes, err := base64URLDecode(parts[1])
	if err != nil {
		return nil, fmt.Errorf("jwt: decode claims: %w", err)
	}

	var claims Claims
	if err := json.Unmarshal(claimsBytes, &claims); err != nil {
		return nil, fmt.Errorf("jwt: unmarshal claims: %w", err)
	}

	sigBytes, err := base64URLDecode(parts[2])
	if err != nil {
		return nil, fmt.Errorf("jwt: decode signature: %w", err)
	}

	return &Token{
		Header:    header,
		Claims:    claims,
		Signature: sigBytes,
		Raw:       tokenStr,
	}, nil
}

// GenerateKeyPair generates a new ECDSA P-256 key pair suitable for ES256 signing.
func GenerateKeyPair() (*ecdsa.PrivateKey, error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("jwt: generate key pair: %w", err)
	}
	return key, nil
}

// JWKS represents a JSON Web Key Set.
type JWKS struct {
	Keys []JWK `json:"keys"`
}

// JWK represents a single JSON Web Key (EC public key).
type JWK struct {
	KTY string `json:"kty"`
	CRV string `json:"crv"`
	X   string `json:"x"`
	Y   string `json:"y"`
	KID string `json:"kid"`
	Use string `json:"use"`
	ALG string `json:"alg"`
}

// JWKSHandler returns an http.HandlerFunc that serves a JWKS JSON response
// at /.well-known/jwks.json for the given public keys.
func JWKSHandler(keys map[string]*ecdsa.PublicKey) http.HandlerFunc {
	jwks := JWKS{Keys: make([]JWK, 0, len(keys))}
	for kid, key := range keys {
		jwks.Keys = append(jwks.Keys, JWK{
			KTY: "EC",
			CRV: "P-256",
			X:   base64URLEncode(key.X.Bytes()),
			Y:   base64URLEncode(key.Y.Bytes()),
			KID: kid,
			Use: "sig",
			ALG: "ES256",
		})
	}

	jwksJSON, _ := json.Marshal(jwks)

	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "public, max-age=3600")
		w.WriteHeader(http.StatusOK)
		w.Write(jwksJSON)
	}
}

// base64URLEncode encodes data using base64url encoding without padding.
func base64URLEncode(data []byte) string {
	return base64.RawURLEncoding.EncodeToString(data)
}

// base64URLDecode decodes a base64url-encoded string without padding.
func base64URLDecode(s string) ([]byte, error) {
	return base64.RawURLEncoding.DecodeString(s)
}
