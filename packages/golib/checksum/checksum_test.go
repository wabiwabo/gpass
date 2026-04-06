package checksum

import (
	"strings"
	"testing"
)

func TestSHA256Sum(t *testing.T) {
	// Known vector: SHA-256("hello") = 2cf24dba...
	sum := SHA256Sum([]byte("hello"))
	if !strings.HasPrefix(sum, "2cf24dba") {
		t.Errorf("SHA256Sum = %q", sum)
	}
	if len(sum) != 64 {
		t.Errorf("len = %d, want 64", len(sum))
	}
}

func TestSHA512Sum(t *testing.T) {
	sum := SHA512Sum([]byte("hello"))
	if len(sum) != 128 {
		t.Errorf("len = %d, want 128", len(sum))
	}
}

func TestSum_Deterministic(t *testing.T) {
	data := []byte("test data")
	s1 := Sum(data, SHA256)
	s2 := Sum(data, SHA256)
	if s1 != s2 {
		t.Error("should be deterministic")
	}
}

func TestSum_DifferentData(t *testing.T) {
	s1 := Sum([]byte("data1"), SHA256)
	s2 := Sum([]byte("data2"), SHA256)
	if s1 == s2 {
		t.Error("different data should produce different checksums")
	}
}

func TestSumReader(t *testing.T) {
	r := strings.NewReader("hello")
	sum, err := SumReader(r, SHA256)
	if err != nil {
		t.Fatalf("SumReader: %v", err)
	}
	expected := SHA256Sum([]byte("hello"))
	if sum != expected {
		t.Errorf("SumReader = %q, want %q", sum, expected)
	}
}

func TestVerify_Valid(t *testing.T) {
	data := []byte("important data")
	sum := SHA256Sum(data)
	if !Verify(data, sum, SHA256) {
		t.Error("valid checksum should verify")
	}
}

func TestVerify_Invalid(t *testing.T) {
	if Verify([]byte("data"), "wrong-checksum", SHA256) {
		t.Error("invalid checksum should not verify")
	}
}

func TestVerify_Tampered(t *testing.T) {
	data := []byte("original")
	sum := SHA256Sum(data)
	if Verify([]byte("tampered"), sum, SHA256) {
		t.Error("tampered data should not verify")
	}
}

func TestVerifyReader_Valid(t *testing.T) {
	data := "test content"
	sum := SHA256Sum([]byte(data))
	ok, err := VerifyReader(strings.NewReader(data), sum, SHA256)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if !ok {
		t.Error("should verify")
	}
}

func TestVerifyReader_Invalid(t *testing.T) {
	ok, err := VerifyReader(strings.NewReader("data"), "wrong", SHA256)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if ok {
		t.Error("should not verify")
	}
}

func TestSHA256_EmptyData(t *testing.T) {
	// SHA-256("") = e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855
	sum := SHA256Sum([]byte{})
	if !strings.HasPrefix(sum, "e3b0c442") {
		t.Errorf("empty SHA256 = %q", sum)
	}
}

func TestSHA512_DifferentFromSHA256(t *testing.T) {
	data := []byte("test")
	s256 := Sum(data, SHA256)
	s512 := Sum(data, SHA512)
	if s256 == s512 {
		t.Error("different algorithms should produce different checksums")
	}
}
