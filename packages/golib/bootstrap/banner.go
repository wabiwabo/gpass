package bootstrap

import (
	"fmt"
	"strings"
)

// BannerConfig holds information to display in the startup banner.
type BannerConfig struct {
	ServiceName string
	Version     string
	Port        string
	Environment string
	GoVersion   string
	Features    []string // enabled features
}

// Banner prints a startup banner with service information.
func Banner(cfg BannerConfig) {
	fmt.Print(FormatBanner(cfg))
}

// FormatBanner returns the banner as a string (for testing).
func FormatBanner(cfg BannerConfig) string {
	features := "none"
	if len(cfg.Features) > 0 {
		features = strings.Join(cfg.Features, ", ")
	}

	lines := []string{
		fmt.Sprintf("  GarudaPass / %s v%s", cfg.ServiceName, cfg.Version),
		fmt.Sprintf("  Port: %s | Env: %s", cfg.Port, cfg.Environment),
		fmt.Sprintf("  Go: %s", cfg.GoVersion),
		fmt.Sprintf("  Features: %s", features),
	}

	// Find the widest line to determine box width.
	maxLen := 0
	for _, line := range lines {
		if len(line) > maxLen {
			maxLen = len(line)
		}
	}

	// Add padding on each side.
	width := maxLen + 2

	var b strings.Builder
	b.WriteString("╔" + strings.Repeat("═", width) + "╗\n")
	for _, line := range lines {
		padding := width - len(line)
		b.WriteString("║" + line + strings.Repeat(" ", padding) + "║\n")
	}
	b.WriteString("╚" + strings.Repeat("═", width) + "╝\n")

	return b.String()
}

// CollectFeatures inspects the service configuration and returns enabled features.
func CollectFeatures(metrics, health, cors, hsts, compression bool) []string {
	var features []string
	if metrics {
		features = append(features, "metrics")
	}
	if health {
		features = append(features, "health")
	}
	if cors {
		features = append(features, "cors")
	}
	if hsts {
		features = append(features, "hsts")
	}
	if compression {
		features = append(features, "compression")
	}
	return features
}
