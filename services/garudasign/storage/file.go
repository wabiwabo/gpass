package storage

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// FileStorage manages file storage within a base directory.
type FileStorage struct {
	baseDir string
}

// NewFileStorage creates a new FileStorage rooted at baseDir.
func NewFileStorage(baseDir string) *FileStorage {
	return &FileStorage{baseDir: baseDir}
}

// Save stores the contents of r as a file with a random prefix prepended to filename.
// It returns the relative path within baseDir.
func (fs *FileStorage) Save(filename string, r io.Reader) (string, error) {
	if strings.Contains(filename, "..") {
		return "", fmt.Errorf("invalid filename: path traversal detected")
	}

	prefix, err := randomPrefix()
	if err != nil {
		return "", fmt.Errorf("generate prefix: %w", err)
	}

	safeName := prefix + "_" + filepath.Base(filename)
	fullPath := filepath.Join(fs.baseDir, safeName)

	if err := os.MkdirAll(filepath.Dir(fullPath), 0o750); err != nil {
		return "", fmt.Errorf("create directory: %w", err)
	}

	f, err := os.Create(fullPath)
	if err != nil {
		return "", fmt.Errorf("create file: %w", err)
	}
	defer f.Close()

	if _, err := io.Copy(f, r); err != nil {
		return "", fmt.Errorf("write file: %w", err)
	}

	return safeName, nil
}

// Load returns a ReadCloser for the file at the given relative path.
func (fs *FileStorage) Load(path string) (io.ReadCloser, error) {
	fullPath := filepath.Join(fs.baseDir, path)
	if err := fs.validatePath(fullPath); err != nil {
		return nil, err
	}
	return os.Open(fullPath)
}

// Delete removes the file at the given relative path.
func (fs *FileStorage) Delete(path string) error {
	fullPath := filepath.Join(fs.baseDir, path)
	if err := fs.validatePath(fullPath); err != nil {
		return err
	}
	return os.Remove(fullPath)
}

func (fs *FileStorage) validatePath(fullPath string) error {
	absBase, err := filepath.Abs(fs.baseDir)
	if err != nil {
		return fmt.Errorf("resolve base dir: %w", err)
	}
	absPath, err := filepath.Abs(fullPath)
	if err != nil {
		return fmt.Errorf("resolve path: %w", err)
	}
	if !strings.HasPrefix(absPath, absBase+string(filepath.Separator)) && absPath != absBase {
		return fmt.Errorf("path %q is outside base directory", fullPath)
	}
	return nil
}

func randomPrefix() (string, error) {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
