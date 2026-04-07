package hash

import (
	"errors"
	"strings"
	"testing"
)

// erroringReader returns an error on the first Read.
type erroringReader struct{}

func (erroringReader) Read(_ []byte) (int, error) { return 0, errors.New("io boom") }

// TestComputeHash_IOError pins the io.Copy error branch.
func TestComputeHash_IOError(t *testing.T) {
	_, err := ComputeHash(erroringReader{})
	if err == nil {
		t.Fatal("expected io error")
	}
}

// TestVerifyHash_IOErrorReturnsFalse pins the err != nil → false branch.
func TestVerifyHash_IOErrorReturnsFalse(t *testing.T) {
	if VerifyHash(erroringReader{}, "abc") {
		t.Error("io error should not verify")
	}
}

// TestVerifyHash_HappyAndMismatch pins both verify outcomes.
func TestVerifyHash_HappyAndMismatch(t *testing.T) {
	expected := "b94d27b9934d3e08a52e52d7da7dabfac484efe37a5380ee9088f7ace2efcde9"
	if !VerifyHash(strings.NewReader("hello world"), expected) {
		t.Error("correct hash should verify")
	}
	if VerifyHash(strings.NewReader("hello world"), "wrong") {
		t.Error("wrong hash should not verify")
	}
}
