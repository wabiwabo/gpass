package storage

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// erroringReader returns an error after the first read.
type erroringReader struct{}

func (erroringReader) Read(_ []byte) (int, error) { return 0, errors.New("io fail") }

// TestSave_PathTraversalRejected pins the security guard against ".."
// in the requested filename.
func TestSave_PathTraversalRejected(t *testing.T) {
	fs := NewFileStorage(t.TempDir())
	for _, name := range []string{"../etc/passwd", "..", "a/../b"} {
		if _, err := fs.Save(name, bytes.NewReader([]byte("x"))); err == nil {
			t.Errorf("traversal accepted: %q", name)
		}
	}
}

// TestSave_IOErrorPropagated pins the io.Copy error branch.
func TestSave_IOErrorPropagated(t *testing.T) {
	fs := NewFileStorage(t.TempDir())
	_, err := fs.Save("doc.pdf", erroringReader{})
	if err == nil || !strings.Contains(err.Error(), "write file") {
		t.Errorf("err = %v", err)
	}
}

// TestSave_HappyPath pins the success branch and confirms the random
// prefix produces a unique filename across calls.
func TestSave_HappyPath(t *testing.T) {
	dir := t.TempDir()
	fs := NewFileStorage(dir)

	p1, err := fs.Save("contract.pdf", bytes.NewReader([]byte("v1")))
	if err != nil {
		t.Fatal(err)
	}
	p2, err := fs.Save("contract.pdf", bytes.NewReader([]byte("v2")))
	if err != nil {
		t.Fatal(err)
	}
	if p1 == p2 {
		t.Errorf("two saves of same filename produced identical paths: %q", p1)
	}
	// Both files must end in the original filename so callers can
	// recover the original name.
	if !strings.HasSuffix(p1, "_contract.pdf") {
		t.Errorf("path %q missing original filename", p1)
	}
	// Files should exist on disk.
	if _, err := os.Stat(filepath.Join(dir, p1)); err != nil {
		t.Errorf("p1 not on disk: %v", err)
	}
}

// TestLoad_AndDelete pins the canonical Load + Delete loop.
func TestLoad_AndDelete(t *testing.T) {
	fs := NewFileStorage(t.TempDir())
	path, _ := fs.Save("a.pdf", bytes.NewReader([]byte("hello")))

	rc, err := fs.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(rc)
	rc.Close()
	if string(body) != "hello" {
		t.Errorf("body = %q", body)
	}

	if err := fs.Delete(path); err != nil {
		t.Errorf("Delete: %v", err)
	}
	// Second Load must fail.
	if _, err := fs.Load(path); err == nil {
		t.Error("Load after Delete should fail")
	}
}

// TestLoad_OutsideBaseRejected pins the validatePath security guard
// — Load with a "../etc/passwd"-style path must be rejected EVEN if
// the file exists outside the base dir.
func TestLoad_OutsideBaseRejected(t *testing.T) {
	fs := NewFileStorage(t.TempDir())
	if _, err := fs.Load("../../../etc/passwd"); err == nil {
		t.Error("path traversal accepted in Load")
	}
}

// TestDelete_OutsideBaseRejected mirrors the Load guard for Delete.
func TestDelete_OutsideBaseRejected(t *testing.T) {
	fs := NewFileStorage(t.TempDir())
	if err := fs.Delete("../../../tmp/anything"); err == nil {
		t.Error("path traversal accepted in Delete")
	}
}
