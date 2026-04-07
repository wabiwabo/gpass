package configloader

import (
	"strings"
	"testing"
	"time"
)

// TestLoad_NestedAndPrefixedStructs covers the env_prefix nested-struct
// branch and the LoadWithPrefix entrypoint, both partially uncovered.
func TestLoad_NestedAndPrefixedStructs(t *testing.T) {
	type DB struct {
		Host string `env:"HOST" default:"localhost"`
		Port int    `env:"PORT" default:"5432"`
	}
	type App struct {
		Name string `env:"NAME"`
		DB   DB     `env_prefix:"DB"`
	}

	t.Setenv("APP_NAME", "garudaaudit")
	t.Setenv("APP_DB_HOST", "postgres.svc")
	// PORT not set → default kicks in.

	var cfg App
	if err := LoadWithPrefix(&cfg, "APP"); err != nil {
		t.Fatalf("LoadWithPrefix: %v", err)
	}
	if cfg.Name != "garudaaudit" {
		t.Errorf("Name = %q", cfg.Name)
	}
	if cfg.DB.Host != "postgres.svc" {
		t.Errorf("DB.Host = %q", cfg.DB.Host)
	}
	if cfg.DB.Port != 5432 {
		t.Errorf("DB.Port = %d, want 5432 (default)", cfg.DB.Port)
	}
}

// TestLoad_TypeParseErrors hits the int/float/bool/duration parse errors
// in setField, plus the unsupported-slice and unsupported-type branches.
func TestLoad_TypeParseErrors(t *testing.T) {
	t.Run("bad int", func(t *testing.T) {
		type C struct {
			N int `env:"BAD_INT"`
		}
		t.Setenv("BAD_INT", "not-a-number")
		err := Load(&C{})
		if err == nil || !strings.Contains(err.Error(), "invalid int") {
			t.Errorf("err = %v", err)
		}
	})
	t.Run("bad float", func(t *testing.T) {
		type C struct {
			F float64 `env:"BAD_FLOAT"`
		}
		t.Setenv("BAD_FLOAT", "x")
		err := Load(&C{})
		if err == nil || !strings.Contains(err.Error(), "invalid float") {
			t.Errorf("err = %v", err)
		}
	})
	t.Run("bad bool", func(t *testing.T) {
		type C struct {
			B bool `env:"BAD_BOOL"`
		}
		t.Setenv("BAD_BOOL", "yesplease")
		err := Load(&C{})
		if err == nil || !strings.Contains(err.Error(), "invalid bool") {
			t.Errorf("err = %v", err)
		}
	})
	t.Run("bad duration", func(t *testing.T) {
		type C struct {
			D time.Duration `env:"BAD_DUR"`
		}
		t.Setenv("BAD_DUR", "5potatoes")
		err := Load(&C{})
		if err == nil || !strings.Contains(err.Error(), "invalid duration") {
			t.Errorf("err = %v", err)
		}
	})
	t.Run("unsupported slice elem", func(t *testing.T) {
		type C struct {
			Ns []int `env:"NS"`
		}
		t.Setenv("NS", "1,2,3")
		err := Load(&C{})
		if err == nil || !strings.Contains(err.Error(), "unsupported slice type") {
			t.Errorf("err = %v", err)
		}
	})
}

// TestLoad_StringSlice covers the comma-separated string-slice path.
func TestLoad_StringSlice(t *testing.T) {
	type C struct {
		Origins []string `env:"ORIGINS"`
	}
	t.Setenv("ORIGINS", "https://a.example , https://b.example,https://c.example")
	var c C
	if err := Load(&c); err != nil {
		t.Fatal(err)
	}
	want := []string{"https://a.example", "https://b.example", "https://c.example"}
	if len(c.Origins) != len(want) {
		t.Fatalf("len = %d, want %d", len(c.Origins), len(want))
	}
	for i := range want {
		if c.Origins[i] != want[i] {
			t.Errorf("Origins[%d] = %q, want %q", i, c.Origins[i], want[i])
		}
	}
}

// TestValidate_MinMaxFailures covers the validate "min=" and "max="
// failure branches against int and float fields.
func TestValidate_MinMaxFailures(t *testing.T) {
	type C struct {
		Port  int     `validate:"min=1024,max=65535"`
		Ratio float64 `validate:"min=0,max=1"`
	}

	if err := Validate(&C{Port: 80, Ratio: 0.5}); err == nil ||
		!strings.Contains(err.Error(), "Port must be >= 1024") {
		t.Errorf("min int: %v", err)
	}
	if err := Validate(&C{Port: 99999, Ratio: 0.5}); err == nil ||
		!strings.Contains(err.Error(), "Port must be <= 65535") {
		t.Errorf("max int: %v", err)
	}
	if err := Validate(&C{Port: 8080, Ratio: 1.5}); err == nil ||
		!strings.Contains(err.Error(), "Ratio must be <= 1") {
		t.Errorf("max float: %v", err)
	}
	if err := Validate(&C{Port: 8080, Ratio: 0.5}); err != nil {
		t.Errorf("valid config: %v", err)
	}
}

// TestDump_RedactsSensitiveAndHandlesNested pins that sensitive fields
// are masked and that nested structs with env_prefix are walked into.
func TestDump_RedactsSensitiveAndHandlesNested(t *testing.T) {
	type Secrets struct {
		APIKey string `env:"KEY" sensitive:"true"`
	}
	type C struct {
		Name    string  `env:"NAME"`
		Secrets Secrets `env_prefix:"SEC"`
	}
	c := C{Name: "audit", Secrets: Secrets{APIKey: "super-secret"}}
	out := Dump(&c)

	if out["NAME"] != "audit" {
		t.Errorf("NAME = %q", out["NAME"])
	}
	if out["SEC_KEY"] != "[REDACTED]" {
		t.Errorf("SEC_KEY = %q, want [REDACTED]", out["SEC_KEY"])
	}
	// Belt-and-braces: the plaintext must NOT appear anywhere in the dump.
	for k, v := range out {
		if strings.Contains(v, "super-secret") {
			t.Errorf("plaintext leaked at %s = %q", k, v)
		}
	}
}
