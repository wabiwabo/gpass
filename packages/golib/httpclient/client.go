package httpclient

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/garudapass/gpass/packages/golib/circuitbreaker"
)

// Client is an HTTP client with optional circuit breaker support.
type Client struct {
	baseURL string
	http    *http.Client
	apiKey  string
	cb      *circuitbreaker.Breaker
}

// Option configures the Client.
type Option func(*Client)

// WithTimeout sets the HTTP client timeout.
func WithTimeout(d time.Duration) Option {
	return func(c *Client) {
		c.http.Timeout = d
	}
}

// WithAPIKey sets the X-API-Key header on all requests.
func WithAPIKey(key string) Option {
	return func(c *Client) {
		c.apiKey = key
	}
}

// WithCircuitBreaker attaches a circuit breaker to the client.
func WithCircuitBreaker(cb *circuitbreaker.Breaker) Option {
	return func(c *Client) {
		c.cb = cb
	}
}

// New creates a new HTTP client with the given base URL and options.
func New(baseURL string, opts ...Option) *Client {
	c := &Client{
		baseURL: baseURL,
		http:    &http.Client{Timeout: 30 * time.Second},
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// Post sends a POST request with a JSON body and decodes the response into result.
func (c *Client) Post(ctx context.Context, path string, body, result interface{}) error {
	data, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("marshal request body: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	return c.do(req, result)
}

// Get sends a GET request and decodes the response into result.
func (c *Client) Get(ctx context.Context, path string, result interface{}) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	return c.do(req, result)
}

func (c *Client) do(req *http.Request, result interface{}) error {
	req.Header.Set("Accept", "application/json")
	if c.apiKey != "" {
		req.Header.Set("X-API-Key", c.apiKey)
	}

	if c.cb != nil && !c.cb.Allow() {
		return errors.New("circuit breaker is open")
	}

	resp, err := c.http.Do(req)
	if err != nil {
		if c.cb != nil {
			c.cb.RecordFailure()
		}
		return fmt.Errorf("execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 500 {
		if c.cb != nil {
			c.cb.RecordFailure()
		}
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("server error: status %d, body: %s", resp.StatusCode, string(body))
	}

	if c.cb != nil {
		c.cb.RecordSuccess()
	}

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("client error: status %d, body: %s", resp.StatusCode, string(body))
	}

	if result != nil {
		if err := json.NewDecoder(resp.Body).Decode(result); err != nil {
			return fmt.Errorf("decode response: %w", err)
		}
	}

	return nil
}
