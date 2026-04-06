package httpmethod

import (
	"testing"
)

func TestIsValid(t *testing.T) {
	tests := []struct {
		method string
		want   bool
	}{
		{"GET", true},
		{"POST", true},
		{"PUT", true},
		{"PATCH", true},
		{"DELETE", true},
		{"HEAD", true},
		{"OPTIONS", true},
		{"TRACE", true},
		{"CONNECT", true},
		{"INVALID", false},
		{"get", false},
		{"", false},
	}
	for _, tt := range tests {
		t.Run(tt.method, func(t *testing.T) {
			if got := IsValid(tt.method); got != tt.want {
				t.Errorf("IsValid(%q) = %v, want %v", tt.method, got, tt.want)
			}
		})
	}
}

func TestIsSafe(t *testing.T) {
	safe := map[string]bool{
		"GET": true, "HEAD": true, "OPTIONS": true, "TRACE": true,
	}
	for _, m := range All() {
		t.Run(m, func(t *testing.T) {
			want := safe[m]
			if got := IsSafe(m); got != want {
				t.Errorf("IsSafe(%q) = %v, want %v", m, got, want)
			}
		})
	}
}

func TestIsIdempotent(t *testing.T) {
	idempotent := map[string]bool{
		"GET": true, "HEAD": true, "PUT": true, "DELETE": true,
		"OPTIONS": true, "TRACE": true,
	}
	for _, m := range All() {
		t.Run(m, func(t *testing.T) {
			want := idempotent[m]
			if got := IsIdempotent(m); got != want {
				t.Errorf("IsIdempotent(%q) = %v, want %v", m, got, want)
			}
		})
	}
}

func TestHasBody(t *testing.T) {
	withBody := map[string]bool{
		"POST": true, "PUT": true, "PATCH": true,
	}
	for _, m := range All() {
		t.Run(m, func(t *testing.T) {
			want := withBody[m]
			if got := HasBody(m); got != want {
				t.Errorf("HasBody(%q) = %v, want %v", m, got, want)
			}
		})
	}
}

func TestIsCacheable(t *testing.T) {
	cacheable := map[string]bool{
		"GET": true, "HEAD": true,
	}
	for _, m := range All() {
		t.Run(m, func(t *testing.T) {
			want := cacheable[m]
			if got := IsCacheable(m); got != want {
				t.Errorf("IsCacheable(%q) = %v, want %v", m, got, want)
			}
		})
	}
}

func TestAll(t *testing.T) {
	all := All()
	if len(all) != 9 {
		t.Errorf("All() length: got %d, want 9", len(all))
	}
}

func TestSafe(t *testing.T) {
	s := Safe()
	if len(s) != 4 {
		t.Errorf("Safe() length: got %d, want 4", len(s))
	}
	for _, m := range s {
		if !IsSafe(m) {
			t.Errorf("%q returned by Safe() but IsSafe() is false", m)
		}
	}
}

func TestUnsafe(t *testing.T) {
	u := Unsafe()
	if len(u) != 5 {
		t.Errorf("Unsafe() length: got %d, want 5", len(u))
	}
	for _, m := range u {
		if IsSafe(m) {
			t.Errorf("%q returned by Unsafe() but IsSafe() is true", m)
		}
	}
}

func TestConstants(t *testing.T) {
	if GET != "GET" {
		t.Error("GET constant mismatch")
	}
	if POST != "POST" {
		t.Error("POST constant mismatch")
	}
	if PUT != "PUT" {
		t.Error("PUT constant mismatch")
	}
	if DELETE != "DELETE" {
		t.Error("DELETE constant mismatch")
	}
}
