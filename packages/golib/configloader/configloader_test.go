package configloader

import (
	"testing"
	"time"
)

type testConfig struct {
	Host     string        `env:"HOST" default:"localhost"`
	Port     int           `env:"PORT" default:"4000"`
	Debug    bool          `env:"DEBUG" default:"false"`
	Rate     float64       `env:"RATE" default:"1.5"`
	Timeout  time.Duration `env:"TIMEOUT" default:"30s"`
	Tags     []string      `env:"TAGS" default:"a,b,c"`
	Required string        `env:"REQUIRED" required:"true"`
}

func TestLoad_Defaults(t *testing.T) {
	t.Setenv("REQUIRED", "value")

	var cfg testConfig
	if err := Load(&cfg); err != nil {
		t.Fatal(err)
	}

	if cfg.Host != "localhost" {
		t.Errorf("Host: got %q, want %q", cfg.Host, "localhost")
	}
	if cfg.Port != 4000 {
		t.Errorf("Port: got %d, want 4000", cfg.Port)
	}
	if cfg.Debug != false {
		t.Error("Debug: should be false")
	}
	if cfg.Rate != 1.5 {
		t.Errorf("Rate: got %f, want 1.5", cfg.Rate)
	}
	if cfg.Timeout != 30*time.Second {
		t.Errorf("Timeout: got %v, want 30s", cfg.Timeout)
	}
	if len(cfg.Tags) != 3 || cfg.Tags[0] != "a" {
		t.Errorf("Tags: got %v", cfg.Tags)
	}
}

func TestLoad_EnvOverridesDefault(t *testing.T) {
	t.Setenv("HOST", "0.0.0.0")
	t.Setenv("PORT", "8080")
	t.Setenv("DEBUG", "true")
	t.Setenv("REQUIRED", "yes")

	var cfg testConfig
	if err := Load(&cfg); err != nil {
		t.Fatal(err)
	}

	if cfg.Host != "0.0.0.0" {
		t.Errorf("Host: got %q", cfg.Host)
	}
	if cfg.Port != 8080 {
		t.Errorf("Port: got %d", cfg.Port)
	}
	if cfg.Debug != true {
		t.Error("Debug: should be true")
	}
}

func TestLoad_RequiredMissing(t *testing.T) {
	var cfg testConfig
	err := Load(&cfg)
	if err == nil {
		t.Error("should fail when required field is missing")
	}
}

func TestLoad_InvalidInt(t *testing.T) {
	t.Setenv("PORT", "not-a-number")
	t.Setenv("REQUIRED", "v")

	var cfg testConfig
	err := Load(&cfg)
	if err == nil {
		t.Error("should fail on invalid int")
	}
}

func TestLoad_InvalidBool(t *testing.T) {
	t.Setenv("DEBUG", "maybe")
	t.Setenv("REQUIRED", "v")

	var cfg testConfig
	err := Load(&cfg)
	if err == nil {
		t.Error("should fail on invalid bool")
	}
}

func TestLoad_InvalidDuration(t *testing.T) {
	t.Setenv("TIMEOUT", "notaduration")
	t.Setenv("REQUIRED", "v")

	var cfg testConfig
	err := Load(&cfg)
	if err == nil {
		t.Error("should fail on invalid duration")
	}
}

func TestLoad_NonPointer(t *testing.T) {
	var cfg testConfig
	err := Load(cfg)
	if err == nil {
		t.Error("should fail on non-pointer")
	}
}

func TestLoad_NilPointer(t *testing.T) {
	err := Load((*testConfig)(nil))
	if err == nil {
		t.Error("should fail on nil pointer")
	}
}

func TestLoadWithPrefix(t *testing.T) {
	t.Setenv("APP_HOST", "prod.example.com")
	t.Setenv("APP_PORT", "443")
	t.Setenv("APP_REQUIRED", "yes")

	var cfg testConfig
	if err := LoadWithPrefix(&cfg, "APP"); err != nil {
		t.Fatal(err)
	}

	if cfg.Host != "prod.example.com" {
		t.Errorf("Host: got %q", cfg.Host)
	}
	if cfg.Port != 443 {
		t.Errorf("Port: got %d", cfg.Port)
	}
}

type validatedConfig struct {
	Name string `env:"NAME" validate:"nonempty"`
	Port int    `env:"PORT" validate:"min=1,max=65535"`
	Rate float64 `env:"RATE" validate:"min=0.1,max=100"`
}

func TestValidate_Nonempty(t *testing.T) {
	cfg := validatedConfig{Name: ""}
	err := Validate(&cfg)
	if err == nil {
		t.Error("should fail on empty nonempty field")
	}
}

func TestValidate_MinMax(t *testing.T) {
	cfg := validatedConfig{Name: "ok", Port: 0, Rate: 1.0}
	err := Validate(&cfg)
	if err == nil {
		t.Error("should fail when Port < 1")
	}

	cfg.Port = 70000
	err = Validate(&cfg)
	if err == nil {
		t.Error("should fail when Port > 65535")
	}

	cfg.Port = 4000
	cfg.Rate = 200.0
	err = Validate(&cfg)
	if err == nil {
		t.Error("should fail when Rate > 100")
	}
}

func TestValidate_AllValid(t *testing.T) {
	cfg := validatedConfig{Name: "svc", Port: 4000, Rate: 1.5}
	err := Validate(&cfg)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

type sensitiveConfig struct {
	Host   string `env:"HOST" default:"localhost"`
	Secret string `env:"SECRET" sensitive:"true"`
}

func TestDump_MasksSensitive(t *testing.T) {
	cfg := sensitiveConfig{Host: "prod.host", Secret: "s3cr3t"}
	dump := Dump(&cfg)

	if dump["HOST"] != "prod.host" {
		t.Errorf("HOST: got %q", dump["HOST"])
	}
	if dump["SECRET"] != "[REDACTED]" {
		t.Errorf("SECRET should be redacted, got %q", dump["SECRET"])
	}
}

func TestLoad_SkipsUntaggedFields(t *testing.T) {
	type cfg struct {
		Tagged   string `env:"TAGGED" default:"yes"`
		Untagged string
	}

	var c cfg
	if err := Load(&c); err != nil {
		t.Fatal(err)
	}
	if c.Tagged != "yes" {
		t.Error("tagged field should be loaded")
	}
	if c.Untagged != "" {
		t.Error("untagged field should remain zero value")
	}
}

func TestLoad_SliceWithSpaces(t *testing.T) {
	t.Setenv("TAGS", " x , y , z ")
	t.Setenv("REQUIRED", "v")

	var cfg testConfig
	if err := Load(&cfg); err != nil {
		t.Fatal(err)
	}

	if len(cfg.Tags) != 3 {
		t.Fatalf("expected 3 tags, got %d", len(cfg.Tags))
	}
	if cfg.Tags[0] != "x" || cfg.Tags[1] != "y" || cfg.Tags[2] != "z" {
		t.Errorf("tags should be trimmed: %v", cfg.Tags)
	}
}
