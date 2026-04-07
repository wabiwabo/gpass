package checksum

import (
	"errors"
	"strings"
	"testing"
)

// erroringReader returns an error after the first read.
type erroringReader struct{}

func (erroringReader) Read(_ []byte) (int, error) { return 0, errors.New("io boom") }

// TestSumReader_PropagatesIOError pins the io.Copy error branch.
func TestSumReader_PropagatesIOError(t *testing.T) {
	_, err := SumReader(erroringReader{}, SHA256)
	if err == nil {
		t.Fatal("expected io error")
	}
}

// TestVerifyReader_PropagatesIOError pins the SumReader error propagation.
func TestVerifyReader_PropagatesIOError(t *testing.T) {
	ok, err := VerifyReader(erroringReader{}, "deadbeef", SHA256)
	if err == nil || ok {
		t.Errorf("ok=%v err=%v, want false+error", ok, err)
	}
}

// TestVerifyReader_HappyPath pins the success branch end-to-end.
func TestVerifyReader_HappyPath(t *testing.T) {
	expected := SHA256Sum([]byte("hello"))
	ok, err := VerifyReader(strings.NewReader("hello"), expected, SHA256)
	if err != nil || !ok {
		t.Errorf("ok=%v err=%v, want true+nil", ok, err)
	}
	// Wrong expected → false but no error.
	ok, err = VerifyReader(strings.NewReader("hello"), "wrong", SHA256)
	if err != nil || ok {
		t.Errorf("mismatch: ok=%v err=%v", ok, err)
	}
}

// TestSumReader_SHA512 pins that SHA512 produces a 128-char hex digest.
func TestSumReader_SHA512(t *testing.T) {
	out, err := SumReader(strings.NewReader("hello"), SHA512)
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 128 {
		t.Errorf("SHA512 hex len = %d, want 128", len(out))
	}
}
