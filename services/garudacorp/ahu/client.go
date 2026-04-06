package ahu

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

// Client is an HTTP client for the AHU API with circuit breaker.
type Client struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
	cb         *circuitBreaker
}

// NewClient creates a new AHU API client.
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

// SearchCompany searches for a company by SK number.
func (c *Client) SearchCompany(ctx context.Context, skNumber string) (*CompanySearchResponse, error) {
	req := CompanySearchRequest{SKNumber: skNumber}
	var resp CompanySearchResponse
	if err := c.doPost(ctx, "/api/v1/company/search", req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// GetOfficers retrieves the officers for a company by SK number.
func (c *Client) GetOfficers(ctx context.Context, skNumber string) ([]Officer, error) {
	var resp OfficersResponse
	if err := c.doGet(ctx, "/api/v1/company/"+skNumber+"/officers", &resp); err != nil {
		return nil, err
	}
	return resp.Officers, nil
}

// GetShareholders retrieves the shareholders for a company by SK number.
func (c *Client) GetShareholders(ctx context.Context, skNumber string) ([]Shareholder, error) {
	var resp ShareholdersResponse
	if err := c.doGet(ctx, "/api/v1/company/"+skNumber+"/shareholders", &resp); err != nil {
		return nil, err
	}
	return resp.Shareholders, nil
}

func (c *Client) doPost(ctx context.Context, path string, body, result interface{}) error {
	if !c.cb.allow() {
		return fmt.Errorf("circuit breaker open: AHU service unavailable")
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

	return c.doRequest(req, result)
}

func (c *Client) doGet(ctx context.Context, path string, result interface{}) error {
	if !c.cb.allow() {
		return fmt.Errorf("circuit breaker open: AHU service unavailable")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	if c.apiKey != "" {
		req.Header.Set("X-API-Key", c.apiKey)
	}

	return c.doRequest(req, result)
}

func (c *Client) doRequest(req *http.Request, result interface{}) error {
	resp, err := c.httpClient.Do(req)
	if err != nil {
		c.cb.recordFailure()
		return fmt.Errorf("AHU request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		c.cb.recordFailure()
		return fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		c.cb.recordFailure()
		return fmt.Errorf("AHU returned status %d: %s", resp.StatusCode, string(respBody))
	}

	c.cb.recordSuccess()

	if err := json.Unmarshal(respBody, result); err != nil {
		return fmt.Errorf("unmarshal response: %w", err)
	}

	return nil
}
