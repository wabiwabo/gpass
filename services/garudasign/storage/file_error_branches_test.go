package storage

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// errReader always fails on Read.
type errReader struct{}

func (errReader) Read(_ []byte) (int, error) { return 0, errors.New("boom") }

// TestSave_IoCopyError pins the io.Copy error wrap.
func TestSave_IoCopyError(t *testing.T) {
	dir := t.TempDir()
	fs := NewFileStorage(dir)
	_, err := fs.Save("d.pdf", errReader{})
	if err == nil || !strings.Contains(err.Error(), "write file") {
		t.Errorf("err = %v", err)
	}
}

// TestSave_CreateError pins the os.Create error wrap by making baseDir
// unwritable for the process.
func TestSave_CreateError(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("root bypasses file mode")
	}
	dir := t.TempDir()
	// Remove write bit from baseDir.
	if err := os.Chmod(dir, 0o500); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	defer os.Chmod(dir, 0o700) // allow cleanup

	fs := NewFileStorage(dir)
	_, err := fs.Save("d.pdf", strings.NewReader("%PDF-1.4"))
	if err == nil {
		t.Error("expected create error")
	}
}

// TestSave_StripsDirectoryComponentsFromFilename pins the filepath.Base
// normalization — a caller-supplied "evil/d.pdf" must be saved as the
// basename only, not nested.
func TestSave_StripsDirectoryComponents(t *testing.T) {
	dir := t.TempDir()
	fs := NewFileStorage(dir)
	name, err := fs.Save("subdir/doc.pdf", strings.NewReader("%PDF-1.4"))
	if err != nil {
		t.Fatalf("save: %v", err)
	}
	if strings.Contains(name, "subdir") {
		t.Errorf("directory component leaked: %s", name)
	}
}

// TestLoad_NonExistent pins the os.Open error that propagates from Load.
func TestLoad_NonExistent(t *testing.T) {
	dir := t.TempDir()
	fs := NewFileStorage(dir)
	_, err := fs.Load("nothing.pdf")
	if err == nil {
		t.Error("expected not-exist error")
	}
}

// TestDelete_NonExistent pins os.Remove failure on missing file.
func TestDelete_NonExistent(t *testing.T) {
	dir := t.TempDir()
	fs := NewFileStorage(dir)
	if err := fs.Delete("nothing.pdf"); err == nil {
		t.Error("expected error")
	}
}

// TestSave_LoadRoundTrip verifies Save+Load produces identical bytes.
func TestSave_LoadRoundTrip(t *testing.T) {
	dir := t.TempDir()
	fs := NewFileStorage(dir)
	want := []byte("%PDF-1.4 roundtrip test")
	name, err := fs.Save("rt.pdf", strings.NewReader(string(want)))
	if err != nil {
		t.Fatalf("save: %v", err)
	}
	rc, err := fs.Load(name)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	defer rc.Close()
	got, _ := io.ReadAll(rc)
	if string(got) != string(want) {
		t.Errorf("roundtrip mismatch")
	}
	// The saved file must be inside baseDir.
	abs, _ := filepath.Abs(filepath.Join(dir, name))
	absBase, _ := filepath.Abs(dir)
	if !strings.HasPrefix(abs, absBase) {
		t.Error("saved path outside baseDir")
	}
}
