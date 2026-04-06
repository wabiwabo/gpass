package hash

import (
	"strings"
	"testing"
)

func TestComputeHash_KnownValue(t *testing.T) {
	// SHA-256 of "hello world"
	expected := "b94d27b9934d3e08a52e52d7da7dabfac484efe37a5380ee9088f7ace2efcde9"
	got, err := ComputeHash(strings.NewReader("hello world"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != expected {
		t.Errorf("expected %s, got %s", expected, got)
	}
}

func TestComputeHash_EmptyInput(t *testing.T) {
	// SHA-256 of ""
	expected := "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
	got, err := ComputeHash(strings.NewReader(""))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != expected {
		t.Errorf("expected %s, got %s", expected, got)
	}
}

func TestVerifyHash_Match(t *testing.T) {
	hash := "b94d27b9934d3e08a52e52d7da7dabfac484efe37a5380ee9088f7ace2efcde9"
	if !VerifyHash(strings.NewReader("hello world"), hash) {
		t.Error("expected hash to match")
	}
}

func TestVerifyHash_Mismatch(t *testing.T) {
	if VerifyHash(strings.NewReader("hello world"), "0000000000000000000000000000000000000000000000000000000000000000") {
		t.Error("expected hash to not match")
	}
}
