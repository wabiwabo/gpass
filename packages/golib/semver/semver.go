// Package semver provides semantic versioning (SemVer 2.0.0) parsing
// and comparison. Supports major.minor.patch format with pre-release
// and build metadata.
package semver

import (
	"fmt"
	"strconv"
	"strings"
)

// Version represents a semantic version.
type Version struct {
	Major      int
	Minor      int
	Patch      int
	PreRelease string
	Build      string
}

// Parse parses a semantic version string.
func Parse(s string) (Version, error) {
	s = strings.TrimPrefix(s, "v")
	if s == "" {
		return Version{}, fmt.Errorf("semver: empty version string")
	}

	var v Version

	// Split off build metadata
	if idx := strings.IndexByte(s, '+'); idx >= 0 {
		v.Build = s[idx+1:]
		s = s[:idx]
	}

	// Split off pre-release
	if idx := strings.IndexByte(s, '-'); idx >= 0 {
		v.PreRelease = s[idx+1:]
		s = s[:idx]
	}

	parts := strings.Split(s, ".")
	if len(parts) != 3 {
		return Version{}, fmt.Errorf("semver: invalid format: %q", s)
	}

	var err error
	v.Major, err = strconv.Atoi(parts[0])
	if err != nil {
		return Version{}, fmt.Errorf("semver: invalid major: %w", err)
	}
	v.Minor, err = strconv.Atoi(parts[1])
	if err != nil {
		return Version{}, fmt.Errorf("semver: invalid minor: %w", err)
	}
	v.Patch, err = strconv.Atoi(parts[2])
	if err != nil {
		return Version{}, fmt.Errorf("semver: invalid patch: %w", err)
	}

	if v.Major < 0 || v.Minor < 0 || v.Patch < 0 {
		return Version{}, fmt.Errorf("semver: negative version component")
	}

	return v, nil
}

// String returns the version string.
func (v Version) String() string {
	s := fmt.Sprintf("%d.%d.%d", v.Major, v.Minor, v.Patch)
	if v.PreRelease != "" {
		s += "-" + v.PreRelease
	}
	if v.Build != "" {
		s += "+" + v.Build
	}
	return s
}

// Compare compares two versions. Returns -1, 0, or 1.
// Pre-release versions have lower precedence.
// Build metadata is ignored.
func Compare(a, b Version) int {
	if d := cmpInt(a.Major, b.Major); d != 0 {
		return d
	}
	if d := cmpInt(a.Minor, b.Minor); d != 0 {
		return d
	}
	if d := cmpInt(a.Patch, b.Patch); d != 0 {
		return d
	}
	// Pre-release has lower precedence
	if a.PreRelease == "" && b.PreRelease == "" {
		return 0
	}
	if a.PreRelease == "" {
		return 1
	}
	if b.PreRelease == "" {
		return -1
	}
	return strings.Compare(a.PreRelease, b.PreRelease)
}

func cmpInt(a, b int) int {
	if a < b {
		return -1
	}
	if a > b {
		return 1
	}
	return 0
}

// Less returns true if a < b.
func Less(a, b Version) bool {
	return Compare(a, b) < 0
}

// Equal returns true if a == b (ignoring build metadata).
func Equal(a, b Version) bool {
	return Compare(a, b) == 0
}

// IsStable returns true if not a pre-release and major > 0.
func (v Version) IsStable() bool {
	return v.Major > 0 && v.PreRelease == ""
}

// IsPreRelease returns true if this is a pre-release version.
func (v Version) IsPreRelease() bool {
	return v.PreRelease != ""
}

// Compatible checks if b is backwards-compatible with a
// (same major, b >= a).
func Compatible(a, b Version) bool {
	return a.Major == b.Major && !Less(b, a)
}

// MustParse parses a version, panicking on error.
func MustParse(s string) Version {
	v, err := Parse(s)
	if err != nil {
		panic(err)
	}
	return v
}
