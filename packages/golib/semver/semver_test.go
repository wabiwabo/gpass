package semver

import (
	"strings"
	"testing"
)

func TestParse(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    Version
		wantErr bool
	}{
		{"simple", "1.2.3", Version{1, 2, 3, "", ""}, false},
		{"v_prefix", "v1.2.3", Version{1, 2, 3, "", ""}, false},
		{"pre_release", "1.0.0-alpha", Version{1, 0, 0, "alpha", ""}, false},
		{"pre_release_num", "1.0.0-beta.1", Version{1, 0, 0, "beta.1", ""}, false},
		{"build", "1.0.0+build.123", Version{1, 0, 0, "", "build.123"}, false},
		{"full", "1.2.3-rc.1+build.456", Version{1, 2, 3, "rc.1", "build.456"}, false},
		{"zeros", "0.0.0", Version{0, 0, 0, "", ""}, false},
		{"empty", "", Version{}, true},
		{"two_parts", "1.2", Version{}, true},
		{"four_parts", "1.2.3.4", Version{}, true},
		{"invalid_major", "x.2.3", Version{}, true},
		{"invalid_minor", "1.y.3", Version{}, true},
		{"invalid_patch", "1.2.z", Version{}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Parse(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("got %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestString(t *testing.T) {
	tests := []struct {
		name string
		v    Version
		want string
	}{
		{"simple", Version{1, 2, 3, "", ""}, "1.2.3"},
		{"pre_release", Version{1, 0, 0, "alpha", ""}, "1.0.0-alpha"},
		{"build", Version{1, 0, 0, "", "build.1"}, "1.0.0+build.1"},
		{"full", Version{2, 1, 0, "rc.1", "20240101"}, "2.1.0-rc.1+20240101"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.v.String(); got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestParseStringRoundTrip(t *testing.T) {
	versions := []string{
		"1.2.3", "0.0.0", "10.20.30",
		"1.0.0-alpha", "1.0.0-alpha.1",
		"1.0.0+build", "1.0.0-rc.1+build.123",
	}
	for _, s := range versions {
		t.Run(s, func(t *testing.T) {
			v, err := Parse(s)
			if err != nil {
				t.Fatalf("parse error: %v", err)
			}
			if got := v.String(); got != s {
				t.Errorf("roundtrip: got %q, want %q", got, s)
			}
		})
	}
}

func TestCompare(t *testing.T) {
	tests := []struct {
		name string
		a, b string
		want int
	}{
		{"equal", "1.0.0", "1.0.0", 0},
		{"major_less", "1.0.0", "2.0.0", -1},
		{"major_greater", "2.0.0", "1.0.0", 1},
		{"minor_less", "1.0.0", "1.1.0", -1},
		{"minor_greater", "1.1.0", "1.0.0", 1},
		{"patch_less", "1.0.0", "1.0.1", -1},
		{"patch_greater", "1.0.1", "1.0.0", 1},
		{"pre_release_lower", "1.0.0-alpha", "1.0.0", -1},
		{"release_higher", "1.0.0", "1.0.0-alpha", 1},
		{"pre_release_compare", "1.0.0-alpha", "1.0.0-beta", -1},
		{"build_ignored", "1.0.0+a", "1.0.0+b", 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := MustParse(tt.a)
			b := MustParse(tt.b)
			if got := Compare(a, b); got != tt.want {
				t.Errorf("Compare(%s, %s) = %d, want %d", tt.a, tt.b, got, tt.want)
			}
		})
	}
}

func TestLess(t *testing.T) {
	if !Less(MustParse("1.0.0"), MustParse("2.0.0")) {
		t.Error("1.0.0 should be less than 2.0.0")
	}
	if Less(MustParse("2.0.0"), MustParse("1.0.0")) {
		t.Error("2.0.0 should not be less than 1.0.0")
	}
	if Less(MustParse("1.0.0"), MustParse("1.0.0")) {
		t.Error("equal versions should not be less")
	}
}

func TestEqual(t *testing.T) {
	if !Equal(MustParse("1.2.3"), MustParse("1.2.3")) {
		t.Error("same versions should be equal")
	}
	if !Equal(MustParse("1.0.0+a"), MustParse("1.0.0+b")) {
		t.Error("build metadata should be ignored in equality")
	}
	if Equal(MustParse("1.0.0"), MustParse("1.0.1")) {
		t.Error("different patches should not be equal")
	}
}

func TestIsStable(t *testing.T) {
	tests := []struct {
		name string
		v    string
		want bool
	}{
		{"stable", "1.0.0", true},
		{"pre_release", "1.0.0-alpha", false},
		{"zero_major", "0.1.0", false},
		{"zero_pre", "0.1.0-beta", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := MustParse(tt.v).IsStable(); got != tt.want {
				t.Errorf("IsStable(%q) = %v, want %v", tt.v, got, tt.want)
			}
		})
	}
}

func TestIsPreRelease(t *testing.T) {
	if !MustParse("1.0.0-alpha").IsPreRelease() {
		t.Error("alpha should be pre-release")
	}
	if MustParse("1.0.0").IsPreRelease() {
		t.Error("release should not be pre-release")
	}
}

func TestCompatible(t *testing.T) {
	tests := []struct {
		name string
		a, b string
		want bool
	}{
		{"same", "1.0.0", "1.0.0", true},
		{"minor_bump", "1.0.0", "1.1.0", true},
		{"patch_bump", "1.0.0", "1.0.1", true},
		{"major_bump", "1.0.0", "2.0.0", false},
		{"downgrade", "1.1.0", "1.0.0", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := MustParse(tt.a)
			b := MustParse(tt.b)
			if got := Compatible(a, b); got != tt.want {
				t.Errorf("Compatible(%s, %s) = %v, want %v", tt.a, tt.b, got, tt.want)
			}
		})
	}
}

func TestMustParsePanics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic")
		}
	}()
	MustParse("invalid")
}

func TestParseNegativeVersion(t *testing.T) {
	_, err := Parse("-1.0.0")
	if err == nil {
		t.Fatal("expected error for negative major")
	}
	if !strings.Contains(err.Error(), "semver:") {
		t.Errorf("error should contain semver prefix: %v", err)
	}
}
