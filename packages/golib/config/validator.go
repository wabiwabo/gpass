package config

import (
	"encoding/hex"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"strconv"
)

// Rule defines a configuration validation rule.
type Rule struct {
	Name     string
	Check    func() error
	Critical bool // if true, service won't start
}

// ValidationError represents a failed validation rule.
type ValidationError struct {
	Rule     string `json:"rule"`
	Critical bool   `json:"critical"`
	Error    string `json:"error"`
}

// Validator checks all configuration rules at startup.
type Validator struct {
	rules  []Rule
	exitFn func(int) // for testing
}

// NewValidator creates a new configuration validator.
func NewValidator() *Validator {
	return &Validator{
		exitFn: os.Exit,
	}
}

// Add adds a validation rule.
func (v *Validator) Add(name string, critical bool, check func() error) *Validator {
	v.rules = append(v.rules, Rule{
		Name:     name,
		Check:    check,
		Critical: critical,
	})
	return v
}

// RequireEnv adds a rule that a specific environment variable must be set.
func (v *Validator) RequireEnv(name string) *Validator {
	return v.Add("env:"+name, true, func() error {
		if os.Getenv(name) == "" {
			return fmt.Errorf("environment variable %s is required but not set", name)
		}
		return nil
	})
}

// RequireURL adds a rule that a URL env var must be a valid URL.
func (v *Validator) RequireURL(name string) *Validator {
	return v.Add("url:"+name, true, func() error {
		val := os.Getenv(name)
		if val == "" {
			return fmt.Errorf("environment variable %s is required but not set", name)
		}
		u, err := url.Parse(val)
		if err != nil {
			return fmt.Errorf("environment variable %s is not a valid URL: %v", name, err)
		}
		if u.Scheme == "" || u.Host == "" {
			return fmt.Errorf("environment variable %s must have a scheme and host", name)
		}
		return nil
	})
}

// RequireHexKey adds a rule that an env var must be a valid hex key of specific byte length.
func (v *Validator) RequireHexKey(name string, byteLen int) *Validator {
	return v.Add("hexkey:"+name, true, func() error {
		val := os.Getenv(name)
		if val == "" {
			return fmt.Errorf("environment variable %s is required but not set", name)
		}
		decoded, err := hex.DecodeString(val)
		if err != nil {
			return fmt.Errorf("environment variable %s is not valid hex: %v", name, err)
		}
		if len(decoded) != byteLen {
			return fmt.Errorf("environment variable %s must be %d bytes, got %d", name, byteLen, len(decoded))
		}
		return nil
	})
}

// RequirePort adds a rule that a port env var must be valid (1-65535).
func (v *Validator) RequirePort(name string) *Validator {
	return v.Add("port:"+name, true, func() error {
		val := os.Getenv(name)
		if val == "" {
			return fmt.Errorf("environment variable %s is required but not set", name)
		}
		port, err := strconv.Atoi(val)
		if err != nil {
			return fmt.Errorf("environment variable %s is not a valid number: %v", name, err)
		}
		if port < 1 || port > 65535 {
			return fmt.Errorf("environment variable %s must be between 1 and 65535, got %d", name, port)
		}
		return nil
	})
}

// Validate runs all rules and returns all errors.
func (v *Validator) Validate() []ValidationError {
	var errs []ValidationError
	for _, rule := range v.rules {
		if err := rule.Check(); err != nil {
			errs = append(errs, ValidationError{
				Rule:     rule.Name,
				Critical: rule.Critical,
				Error:    err.Error(),
			})
		}
	}
	return errs
}

// MustValidate runs all rules and exits if any critical rule fails.
func (v *Validator) MustValidate() {
	errs := v.Validate()
	if len(errs) == 0 {
		return
	}

	hasCritical := false
	for _, e := range errs {
		level := slog.LevelWarn
		if e.Critical {
			level = slog.LevelError
			hasCritical = true
		}
		slog.Log(nil, level, "configuration validation failed",
			"rule", e.Rule,
			"critical", e.Critical,
			"error", e.Error,
		)
	}

	if hasCritical {
		slog.Error("critical configuration errors found, exiting")
		v.exitFn(1)
	}
}
