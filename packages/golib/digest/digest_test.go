package digest

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSHA256(t *testing.T) {
	// Known SHA-256 of "hello"
	hash := SHA256([]byte("hello"))
	expected := "2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824"
	if hash != expected {
		t.Errorf("SHA256('hello') = %s, want %s", hash, expected)
	}
}

func TestSHA256_Empty(t *testing.T) {
	hash := SHA256([]byte{})
	expected := "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
	if hash != expected {
		t.Errorf("SHA256('') = %s, want %s", hash, expected)
	}
}

func TestSHA256String(t *testing.T) {
	hash := SHA256String("hello")
	expected := "2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824"
	if hash != expected {
		t.Errorf("got %s", hash)
	}
}

func TestSHA384(t *testing.T) {
	hash := SHA384([]byte("hello"))
	if len(hash) != 96 { // 384 bits = 48 bytes = 96 hex chars
		t.Errorf("SHA384 length: got %d, want 96", len(hash))
	}
}

func TestSHA512Hash(t *testing.T) {
	hash := SHA512Hash([]byte("hello"))
	if len(hash) != 128 { // 512 bits = 64 bytes = 128 hex chars
		t.Errorf("SHA512 length: got %d, want 128", len(hash))
	}
}

func TestSHA256Reader(t *testing.T) {
	r := bytes.NewReader([]byte("hello"))
	hash, err := SHA256Reader(r)
	if err != nil {
		t.Fatal(err)
	}
	expected := "2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824"
	if hash != expected {
		t.Errorf("got %s", hash)
	}
}

func TestSHA256File(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.txt")
	os.WriteFile(path, []byte("hello"), 0644)

	hash, err := SHA256File(path)
	if err != nil {
		t.Fatal(err)
	}
	expected := "2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824"
	if hash != expected {
		t.Errorf("got %s", hash)
	}
}

func TestSHA256File_NotFound(t *testing.T) {
	_, err := SHA256File("/nonexistent/file")
	if err == nil {
		t.Error("should fail for missing file")
	}
}

func TestCRC32(t *testing.T) {
	c := CRC32([]byte("hello"))
	if c == 0 {
		t.Error("CRC32 should not be 0 for non-empty data")
	}
}

func TestCRC32Hex(t *testing.T) {
	hex := CRC32Hex([]byte("hello"))
	if len(hex) != 8 {
		t.Errorf("CRC32 hex length: got %d, want 8", len(hex))
	}
}

func TestVerify_Match(t *testing.T) {
	data := []byte("test data for verification")
	hash := SHA256(data)

	if !Verify(data, hash) {
		t.Error("verify should return true for matching data")
	}
}

func TestVerify_Mismatch(t *testing.T) {
	if Verify([]byte("original"), SHA256([]byte("modified"))) {
		t.Error("verify should return false for mismatched data")
	}
}

func TestVerifyFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "doc.pdf")
	data := []byte("PDF content for verification")
	os.WriteFile(path, data, 0644)

	hash := SHA256(data)
	ok, err := VerifyFile(path, hash)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Error("file should verify")
	}
}

func TestVerifyFile_Tampered(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "doc.pdf")
	os.WriteFile(path, []byte("original"), 0644)

	hash := SHA256([]byte("original"))

	// Tamper
	os.WriteFile(path, []byte("tampered"), 0644)

	ok, err := VerifyFile(path, hash)
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Error("tampered file should not verify")
	}
}

func TestComputeMulti(t *testing.T) {
	data := []byte("multi-hash test")
	mh := ComputeMulti(data)

	if mh.SHA256 == "" {
		t.Error("SHA256 should not be empty")
	}
	if mh.SHA384 == "" {
		t.Error("SHA384 should not be empty")
	}
	if mh.SHA512 == "" {
		t.Error("SHA512 should not be empty")
	}
	if mh.CRC32 == "" {
		t.Error("CRC32 should not be empty")
	}
	if mh.Size != int64(len(data)) {
		t.Errorf("size: got %d, want %d", mh.Size, len(data))
	}

	// Cross-check.
	if mh.SHA256 != SHA256(data) {
		t.Error("SHA256 mismatch")
	}
}

func TestComputeMultiReader(t *testing.T) {
	data := "reader multi-hash test"
	r := strings.NewReader(data)
	mh, err := ComputeMultiReader(r)
	if err != nil {
		t.Fatal(err)
	}

	if mh.SHA256 != SHA256([]byte(data)) {
		t.Error("SHA256 mismatch from reader")
	}
	if mh.Size != int64(len(data)) {
		t.Errorf("size: got %d", mh.Size)
	}
}

func TestSHA256_Deterministic(t *testing.T) {
	data := []byte("deterministic test")
	h1 := SHA256(data)
	h2 := SHA256(data)
	if h1 != h2 {
		t.Error("SHA256 should be deterministic")
	}
}

func TestSHA256_Different(t *testing.T) {
	h1 := SHA256([]byte("data1"))
	h2 := SHA256([]byte("data2"))
	if h1 == h2 {
		t.Error("different data should produce different hashes")
	}
}
