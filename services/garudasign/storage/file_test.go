package storage

import (
	"io"
	"os"
	"strings"
	"testing"
)

func TestSaveAndLoad(t *testing.T) {
	dir := t.TempDir()
	fs := NewFileStorage(dir)

	content := "test file content"
	path, err := fs.Save("test.pdf", strings.NewReader(content))
	if err != nil {
		t.Fatalf("save: %v", err)
	}

	rc, err := fs.Load(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	defer rc.Close()

	data, err := io.ReadAll(rc)
	if err != nil {
		t.Fatalf("read: %v", err)
	}

	if string(data) != content {
		t.Errorf("expected %q, got %q", content, string(data))
	}
}

func TestDelete(t *testing.T) {
	dir := t.TempDir()
	fs := NewFileStorage(dir)

	path, err := fs.Save("delete-me.pdf", strings.NewReader("data"))
	if err != nil {
		t.Fatalf("save: %v", err)
	}

	if err := fs.Delete(path); err != nil {
		t.Fatalf("delete: %v", err)
	}

	_, err = fs.Load(path)
	if err == nil {
		t.Error("expected error loading deleted file")
	}
}

func TestSave_PathTraversal(t *testing.T) {
	dir := t.TempDir()
	fs := NewFileStorage(dir)

	_, err := fs.Save("../etc/passwd", strings.NewReader("evil"))
	if err == nil {
		t.Error("expected error for path traversal")
	}
}

func TestLoad_Nonexistent(t *testing.T) {
	dir := t.TempDir()
	fs := NewFileStorage(dir)

	_, err := fs.Load("does-not-exist.pdf")
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

func TestLoad_OutsideDir(t *testing.T) {
	dir := t.TempDir()
	fs := NewFileStorage(dir)

	// Create a file outside the base dir
	outside, err := os.CreateTemp("", "outside-*")
	if err != nil {
		t.Fatalf("create temp: %v", err)
	}
	defer os.Remove(outside.Name())
	outside.Close()

	_, err = fs.Load("../../" + outside.Name())
	if err == nil {
		t.Error("expected error for path outside base directory")
	}
}
