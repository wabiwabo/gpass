package redis

import (
	"context"
	"strings"
	"testing"
)

// TestValidate_BadURL covers the url.Parse error and the scheme-mismatch
// branches that the existing tests didn't reach.
func TestValidate_BadURL(t *testing.T) {
	cases := []struct {
		name string
		url  string
		want string
	}{
		{"empty", "", "URL is required"},
		// A URL with a control byte fails url.Parse.
		{"bad parse", "redis://exa\x00mple.com", "invalid URL"},
		// http:// is parseable but the wrong scheme.
		{"wrong scheme http", "http://example.com:6379", "must be redis:// or rediss://"},
		{"wrong scheme tcp", "tcp://example.com:6379", "must be redis:// or rediss://"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := Config{URL: tc.url}
			err := c.Validate()
			if err == nil {
				t.Fatal("expected error")
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("err = %q, want substring %q", err.Error(), tc.want)
			}
		})
	}
}

// TestValidate_AcceptsBothSchemes pins that both redis:// and rediss://
// (TLS variant) are accepted.
func TestValidate_AcceptsBothSchemes(t *testing.T) {
	for _, u := range []string{
		"redis://localhost:6379",
		"rediss://localhost:6379",
		"redis://:password@host:6379/0",
	} {
		c := Config{URL: u}
		if err := c.Validate(); err != nil {
			t.Errorf("Validate(%q) = %v, want nil", u, err)
		}
	}
}

// TestClient_HealthAndClose covers the previously-0% Health and Close
// stub branches. They're stubs in the current code (no real Redis
// driver wired in) but still need to be reachable so callers don't
// panic on a nil method dispatch.
func TestClient_HealthAndClose(t *testing.T) {
	c, err := New(Config{URL: "redis://localhost:6379"})
	if err != nil {
		t.Fatal(err)
	}
	if err := c.Health(context.Background()); err != nil {
		t.Errorf("Health = %v, want nil", err)
	}
	if err := c.Close(); err != nil {
		t.Errorf("Close = %v, want nil", err)
	}
}

// TestNew_AppliesDefaults pins that New populates all the default
// timeout/retry/pool values when an empty config is passed.
func TestNew_AppliesDefaults(t *testing.T) {
	c, err := New(Config{URL: "redis://localhost:6379"})
	if err != nil {
		t.Fatal(err)
	}
	if c.cfg.MaxRetries != 3 {
		t.Errorf("MaxRetries = %d, want 3", c.cfg.MaxRetries)
	}
	if c.cfg.PoolSize != 10 {
		t.Errorf("PoolSize = %d, want 10", c.cfg.PoolSize)
	}
	if c.cfg.DialTimeout == 0 || c.cfg.ReadTimeout == 0 || c.cfg.WriteTimeout == 0 {
		t.Errorf("default timeouts not applied: %+v", c.cfg)
	}
}
