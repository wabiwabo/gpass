package oauth2

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
)

// JWK represents a JSON Web Key.
type JWK struct {
	KID       string `json:"kid"`
	KeyType   string `json:"kty"` // RSA, EC
	Algorithm string `json:"alg"` // RS256, ES256
	Use       string `json:"use"` // sig
	N         string `json:"n,omitempty"`   // RSA modulus
	E         string `json:"e,omitempty"`   // RSA exponent
	X         string `json:"x,omitempty"`   // EC x coordinate
	Y         string `json:"y,omitempty"`   // EC y coordinate
	Crv       string `json:"crv,omitempty"` // EC curve (P-256)
}

// JWKS represents a JSON Web Key Set.
type JWKS struct {
	Keys []JWK `json:"keys"`
}

// JWKSClient fetches and caches JWKS from an OpenID Connect provider.
type JWKSClient struct {
	endpoint   string
	keys       map[string]*JWK // kid -> key
	mu         sync.RWMutex
	lastFetch  time.Time
	cacheTTL   time.Duration
	httpClient *http.Client
}

// NewJWKSClient creates a JWKS client with caching.
func NewJWKSClient(endpoint string, cacheTTL time.Duration) *JWKSClient {
	return &JWKSClient{
		endpoint: endpoint,
		keys:     make(map[string]*JWK),
		cacheTTL: cacheTTL,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// GetKey retrieves a key by kid, refreshing cache if needed.
func (c *JWKSClient) GetKey(ctx context.Context, kid string) (*JWK, error) {
	c.mu.RLock()
	if time.Since(c.lastFetch) < c.cacheTTL {
		if key, ok := c.keys[kid]; ok {
			c.mu.RUnlock()
			return key, nil
		}
	}
	c.mu.RUnlock()

	// Cache miss or expired — refresh.
	if err := c.Refresh(ctx); err != nil {
		return nil, err
	}

	c.mu.RLock()
	defer c.mu.RUnlock()

	key, ok := c.keys[kid]
	if !ok {
		return nil, fmt.Errorf("key with kid %q not found in JWKS", kid)
	}
	return key, nil
}

// Refresh forces a cache refresh from the JWKS endpoint.
func (c *JWKSClient) Refresh(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.endpoint, nil)
	if err != nil {
		return fmt.Errorf("creating JWKS request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("JWKS request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("JWKS endpoint returned status %d", resp.StatusCode)
	}

	var jwks JWKS
	if err := json.NewDecoder(resp.Body).Decode(&jwks); err != nil {
		return fmt.Errorf("decoding JWKS response: %w", err)
	}

	newKeys := make(map[string]*JWK, len(jwks.Keys))
	for i := range jwks.Keys {
		k := jwks.Keys[i]
		newKeys[k.KID] = &k
	}

	c.mu.Lock()
	c.keys = newKeys
	c.lastFetch = time.Now()
	c.mu.Unlock()

	return nil
}
