package environ

import (
	"testing"
	"time"
)

func TestGet(t *testing.T) {
	t.Setenv("TEST_VAR", "hello")
	if Get("TEST_VAR", "default") != "hello" {
		t.Error("should return env value")
	}
	if Get("NONEXISTENT", "fallback") != "fallback" {
		t.Error("should return default")
	}
}

func TestGetInt(t *testing.T) {
	t.Setenv("PORT", "8080")
	if GetInt("PORT", 3000) != 8080 {
		t.Error("should parse int")
	}
	if GetInt("MISSING", 3000) != 3000 {
		t.Error("should return default")
	}
	t.Setenv("BAD", "abc")
	if GetInt("BAD", 3000) != 3000 {
		t.Error("should return default on parse error")
	}
}

func TestGetBool(t *testing.T) {
	t.Setenv("DEBUG", "true")
	if !GetBool("DEBUG", false) {
		t.Error("should parse true")
	}
	t.Setenv("FEATURE", "false")
	if GetBool("FEATURE", true) {
		t.Error("should parse false")
	}
	if !GetBool("MISSING", true) {
		t.Error("should return default")
	}
}

func TestGetDuration(t *testing.T) {
	t.Setenv("TIMEOUT", "30s")
	if GetDuration("TIMEOUT", time.Second) != 30*time.Second {
		t.Error("should parse duration")
	}
	if GetDuration("MISSING", 5*time.Second) != 5*time.Second {
		t.Error("should return default")
	}
	t.Setenv("BAD_DUR", "notaduration")
	if GetDuration("BAD_DUR", time.Second) != time.Second {
		t.Error("should return default on parse error")
	}
}

func TestGetSlice(t *testing.T) {
	t.Setenv("HOSTS", "a, b, c")
	s := GetSlice("HOSTS", nil)
	if len(s) != 3 || s[0] != "a" || s[1] != "b" || s[2] != "c" {
		t.Errorf("got %v", s)
	}
	def := GetSlice("MISSING", []string{"x"})
	if len(def) != 1 || def[0] != "x" {
		t.Error("should return default")
	}
}

func TestRequire_Present(t *testing.T) {
	t.Setenv("REQUIRED", "value")
	if Require("REQUIRED") != "value" {
		t.Error("should return value")
	}
}

func TestRequire_Missing(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("should panic on missing required var")
		}
	}()
	Require("DEFINITELY_MISSING_" + time.Now().String())
}

func TestIsDevelopment(t *testing.T) {
	t.Setenv("ENVIRONMENT", "development")
	if !IsDevelopment() {
		t.Error("should be development")
	}
	t.Setenv("ENVIRONMENT", "production")
	if IsDevelopment() {
		t.Error("production is not development")
	}
}

func TestIsProduction(t *testing.T) {
	t.Setenv("ENVIRONMENT", "production")
	if !IsProduction() {
		t.Error("should be production")
	}
}

func TestIsTest(t *testing.T) {
	t.Setenv("ENVIRONMENT", "test")
	if !IsTest() {
		t.Error("should be test")
	}
}
