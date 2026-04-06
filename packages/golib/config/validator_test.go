package config

import (
	"errors"
	"os"
	"testing"
)

func TestValidator_AllRulesPass(t *testing.T) {
	v := NewValidator()
	v.Add("check1", false, func() error { return nil })
	v.Add("check2", true, func() error { return nil })

	errs := v.Validate()
	if len(errs) != 0 {
		t.Errorf("expected no errors, got %d", len(errs))
	}
}

func TestValidator_RequireEnv_FailsWhenNotSet(t *testing.T) {
	os.Unsetenv("TEST_REQUIRED_VAR")

	v := NewValidator()
	v.RequireEnv("TEST_REQUIRED_VAR")

	errs := v.Validate()
	if len(errs) != 1 {
		t.Fatalf("expected 1 error, got %d", len(errs))
	}
	if errs[0].Rule != "env:TEST_REQUIRED_VAR" {
		t.Errorf("expected rule env:TEST_REQUIRED_VAR, got %s", errs[0].Rule)
	}
	if !errs[0].Critical {
		t.Error("RequireEnv should be critical")
	}
}

func TestValidator_RequireEnv_PassesWhenSet(t *testing.T) {
	t.Setenv("TEST_REQUIRED_VAR", "some-value")

	v := NewValidator()
	v.RequireEnv("TEST_REQUIRED_VAR")

	errs := v.Validate()
	if len(errs) != 0 {
		t.Errorf("expected no errors, got %d", len(errs))
	}
}

func TestValidator_RequireURL_FailsForInvalidURL(t *testing.T) {
	t.Setenv("TEST_URL_VAR", "not-a-url")

	v := NewValidator()
	v.RequireURL("TEST_URL_VAR")

	errs := v.Validate()
	if len(errs) != 1 {
		t.Fatalf("expected 1 error, got %d", len(errs))
	}
	if errs[0].Rule != "url:TEST_URL_VAR" {
		t.Errorf("expected rule url:TEST_URL_VAR, got %s", errs[0].Rule)
	}
}

func TestValidator_RequireURL_PassesForValidURL(t *testing.T) {
	t.Setenv("TEST_URL_VAR", "https://example.com")

	v := NewValidator()
	v.RequireURL("TEST_URL_VAR")

	errs := v.Validate()
	if len(errs) != 0 {
		t.Errorf("expected no errors, got %d", len(errs))
	}
}

func TestValidator_RequireHexKey_FailsForWrongLength(t *testing.T) {
	t.Setenv("TEST_HEX_KEY", "aabbccdd") // 4 bytes

	v := NewValidator()
	v.RequireHexKey("TEST_HEX_KEY", 32) // require 32 bytes

	errs := v.Validate()
	if len(errs) != 1 {
		t.Fatalf("expected 1 error, got %d", len(errs))
	}
	if errs[0].Rule != "hexkey:TEST_HEX_KEY" {
		t.Errorf("expected rule hexkey:TEST_HEX_KEY, got %s", errs[0].Rule)
	}
}

func TestValidator_RequireHexKey_FailsForInvalidHex(t *testing.T) {
	t.Setenv("TEST_HEX_KEY", "not-hex-at-all")

	v := NewValidator()
	v.RequireHexKey("TEST_HEX_KEY", 16)

	errs := v.Validate()
	if len(errs) != 1 {
		t.Fatalf("expected 1 error, got %d", len(errs))
	}
}

func TestValidator_RequireHexKey_Passes(t *testing.T) {
	t.Setenv("TEST_HEX_KEY", "0123456789abcdef0123456789abcdef") // 16 bytes

	v := NewValidator()
	v.RequireHexKey("TEST_HEX_KEY", 16)

	errs := v.Validate()
	if len(errs) != 0 {
		t.Errorf("expected no errors, got %d", len(errs))
	}
}

func TestValidator_RequirePort_FailsForInvalidPort(t *testing.T) {
	tests := []struct {
		name  string
		value string
	}{
		{"zero", "0"},
		{"negative", "-1"},
		{"too large", "70000"},
		{"not a number", "abc"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("TEST_PORT", tt.value)

			v := NewValidator()
			v.RequirePort("TEST_PORT")

			errs := v.Validate()
			if len(errs) != 1 {
				t.Fatalf("expected 1 error, got %d", len(errs))
			}
		})
	}
}

func TestValidator_RequirePort_Passes(t *testing.T) {
	t.Setenv("TEST_PORT", "8080")

	v := NewValidator()
	v.RequirePort("TEST_PORT")

	errs := v.Validate()
	if len(errs) != 0 {
		t.Errorf("expected no errors, got %d", len(errs))
	}
}

func TestValidator_NonCriticalFailureDoesNotExit(t *testing.T) {
	exitCalled := false
	v := NewValidator()
	v.exitFn = func(code int) { exitCalled = true }

	v.Add("non-critical", false, func() error {
		return errors.New("warning")
	})

	v.MustValidate()

	if exitCalled {
		t.Error("os.Exit should not have been called for non-critical failure")
	}
}

func TestValidator_CriticalFailureExits(t *testing.T) {
	exitCalled := false
	exitCode := 0
	v := NewValidator()
	v.exitFn = func(code int) {
		exitCalled = true
		exitCode = code
	}

	v.Add("critical-rule", true, func() error {
		return errors.New("fatal error")
	})

	v.MustValidate()

	if !exitCalled {
		t.Error("os.Exit should have been called for critical failure")
	}
	if exitCode != 1 {
		t.Errorf("expected exit code 1, got %d", exitCode)
	}
}

func TestValidator_MultipleFailuresCollected(t *testing.T) {
	v := NewValidator()
	v.Add("rule1", true, func() error { return errors.New("error 1") })
	v.Add("rule2", false, func() error { return errors.New("error 2") })
	v.Add("rule3", true, func() error { return nil }) // passes

	errs := v.Validate()
	if len(errs) != 2 {
		t.Fatalf("expected 2 errors, got %d", len(errs))
	}

	if errs[0].Rule != "rule1" || errs[1].Rule != "rule2" {
		t.Errorf("unexpected rules: %s, %s", errs[0].Rule, errs[1].Rule)
	}
}

func TestValidator_CustomRuleCheck(t *testing.T) {
	v := NewValidator()

	called := false
	v.Add("custom", false, func() error {
		called = true
		return nil
	})

	errs := v.Validate()
	if !called {
		t.Error("custom check function was not called")
	}
	if len(errs) != 0 {
		t.Errorf("expected no errors, got %d", len(errs))
	}
}
