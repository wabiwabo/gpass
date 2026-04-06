package oauth2

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// TokenInfo contains introspection result per RFC 7662.
type TokenInfo struct {
	Active    bool   `json:"active"`
	Scope     string `json:"scope,omitempty"`
	ClientID  string `json:"client_id,omitempty"`
	Username  string `json:"username,omitempty"`
	TokenType string `json:"token_type,omitempty"`
	ExpiresAt int64  `json:"exp,omitempty"`
	IssuedAt  int64  `json:"iat,omitempty"`
	Subject   string `json:"sub,omitempty"`
	Audience  string `json:"aud,omitempty"`
	Issuer    string `json:"iss,omitempty"`
}

// TokenIntrospector defines the interface for token introspection.
type TokenIntrospector interface {
	Introspect(ctx context.Context, token string) (*TokenInfo, error)
}

// Introspector validates tokens against an OAuth2 authorization server.
type Introspector struct {
	endpoint     string // introspection endpoint URL
	clientID     string
	clientSecret string
	httpClient   *http.Client
}

// NewIntrospector creates a token introspector.
func NewIntrospector(endpoint, clientID, clientSecret string) *Introspector {
	return &Introspector{
		endpoint:     endpoint,
		clientID:     clientID,
		clientSecret: clientSecret,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// Introspect validates a token per RFC 7662.
func (i *Introspector) Introspect(ctx context.Context, token string) (*TokenInfo, error) {
	form := url.Values{}
	form.Set("token", token)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, i.endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("creating introspection request: %w", err)
	}

	req.SetBasicAuth(i.clientID, i.clientSecret)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	body := form.Encode()
	req.Body = io.NopCloser(strings.NewReader(body))
	req.ContentLength = int64(len(body))

	resp, err := i.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("introspection request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("introspection endpoint returned status %d", resp.StatusCode)
	}

	var info TokenInfo
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return nil, fmt.Errorf("decoding introspection response: %w", err)
	}

	return &info, nil
}

// MockIntrospector returns configurable token info for testing.
type MockIntrospector struct {
	Tokens map[string]*TokenInfo // token -> info
}

// NewMockIntrospector creates a new mock introspector.
func NewMockIntrospector() *MockIntrospector {
	return &MockIntrospector{
		Tokens: make(map[string]*TokenInfo),
	}
}

// Introspect returns the configured token info or inactive if unknown.
func (m *MockIntrospector) Introspect(_ context.Context, token string) (*TokenInfo, error) {
	if info, ok := m.Tokens[token]; ok {
		return info, nil
	}
	return &TokenInfo{Active: false}, nil
}

// AddToken adds a token with its info to the mock.
func (m *MockIntrospector) AddToken(token string, info *TokenInfo) {
	m.Tokens[token] = info
}
