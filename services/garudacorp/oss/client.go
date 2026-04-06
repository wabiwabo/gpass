package oss

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

// Circuit breaker states.
const (
	stateClosed   = iota
	stateOpen
	stateHalfOpen
)

// circuitBreaker implements a simple circuit breaker pattern.
type circuitBreaker struct {
	mu           sync.Mutex
	state        int
	failureCount int
	threshold    int
	openUntil    time.Time
	cooldown     time.Duration
}

func newCircuitBreaker(threshold int, cooldown time.Duration) *circuitBreaker {
	return &circuitBreaker{
		state:     stateClosed,
		threshold: threshold,
		cooldown:  cooldown,
	}
}

func (cb *circuitBreaker) allow() bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	switch cb.state {
	case stateClosed:
		return true
	case stateOpen:
		if time.Now().After(cb.openUntil) {
			cb.state = stateHalfOpen
			return true
		}
		return false
	case stateHalfOpen:
		return true
	}
	return false
}

func (cb *circuitBreaker) recordSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.failureCount = 0
	cb.state = stateClosed
}

func (cb *circuitBreaker) recordFailure() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.failureCount++
	if cb.failureCount >= cb.threshold {
		cb.state = stateOpen
		cb.openUntil = time.Now().Add(cb.cooldown)
	}
}

// Client is an HTTP client for the OSS API with circuit breaker.
type Client struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
	cb         *circuitBreaker
}

// NewClient creates a new OSS API client.
func NewClient(baseURL, apiKey string, timeout time.Duration) *Client {
	return &Client{
		baseURL: baseURL,
		apiKey:  apiKey,
		httpClient: &http.Client{
			Timeout: timeout,
		},
		cb: newCircuitBreaker(5, 30*time.Second),
	}
}

// SearchByNPWP searches for a NIB by NPWP.
func (c *Client) SearchByNPWP(ctx context.Context, npwp string) (*NIBSearchResponse, error) {
	req := NIBSearchRequest{NPWP: npwp}
	var resp NIBSearchResponse
	if err := c.doPost(ctx, "/api/v1/nib/search", req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// SearchByNIB searches for business data by NIB.
func (c *Client) SearchByNIB(ctx context.Context, nib string) (*NIBSearchResponse, error) {
	req := NIBSearchRequest{NIB: nib}
	var resp NIBSearchResponse
	if err := c.doPost(ctx, "/api/v1/nib/search", req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Client) doPost(ctx context.Context, path string, body, result interface{}) error {
	if !c.cb.allow() {
		return fmt.Errorf("circuit breaker open: OSS service unavailable")
	}

	payload, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		req.Header.Set("X-API-Key", c.apiKey)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		c.cb.recordFailure()
		return fmt.Errorf("OSS request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		c.cb.recordFailure()
		return fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		c.cb.recordFailure()
		return fmt.Errorf("OSS returned status %d: %s", resp.StatusCode, string(respBody))
	}

	c.cb.recordSuccess()

	if err := json.Unmarshal(respBody, result); err != nil {
		return fmt.Errorf("unmarshal response: %w", err)
	}

	return nil
}
