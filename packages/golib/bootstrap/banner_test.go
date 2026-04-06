package bootstrap

import (
	"strings"
	"testing"
)

func TestFormatBanner_IncludesServiceNameAndVersion(t *testing.T) {
	banner := FormatBanner(BannerConfig{
		ServiceName: "garudasign",
		Version:     "0.1.0",
		Port:        "4007",
		Environment: "production",
		GoVersion:   "1.25.0",
		Features:    []string{"metrics", "health"},
	})

	if !strings.Contains(banner, "garudasign") {
		t.Error("banner should contain service name")
	}
	if !strings.Contains(banner, "v0.1.0") {
		t.Error("banner should contain version")
	}
}

func TestFormatBanner_IncludesPortAndEnvironment(t *testing.T) {
	banner := FormatBanner(BannerConfig{
		ServiceName: "garudasign",
		Version:     "0.1.0",
		Port:        "4007",
		Environment: "production",
		GoVersion:   "1.25.0",
	})

	if !strings.Contains(banner, "Port: 4007") {
		t.Error("banner should contain port")
	}
	if !strings.Contains(banner, "Env: production") {
		t.Error("banner should contain environment")
	}
}

func TestCollectFeatures_ReturnsEnabledOnly(t *testing.T) {
	features := CollectFeatures(true, true, false, true, false)

	expected := []string{"metrics", "health", "hsts"}
	if len(features) != len(expected) {
		t.Fatalf("got %d features, want %d", len(features), len(expected))
	}
	for i, f := range features {
		if f != expected[i] {
			t.Errorf("features[%d] = %q, want %q", i, f, expected[i])
		}
	}
}

func TestFormatBanner_EmptyFeatures(t *testing.T) {
	banner := FormatBanner(BannerConfig{
		ServiceName: "garudasign",
		Version:     "0.1.0",
		Port:        "4007",
		Environment: "dev",
		GoVersion:   "1.25.0",
		Features:    nil,
	})

	if !strings.Contains(banner, "Features: none") {
		t.Error("banner should show 'none' for empty features")
	}
}

func TestBanner_DoesNotPanic(t *testing.T) {
	// Just verify Banner doesn't panic with valid config.
	Banner(BannerConfig{
		ServiceName: "test",
		Version:     "0.0.1",
		Port:        "8080",
		Environment: "dev",
		GoVersion:   "1.25.0",
		Features:    []string{"health"},
	})

	// Also verify with zero-value config.
	Banner(BannerConfig{})
}
