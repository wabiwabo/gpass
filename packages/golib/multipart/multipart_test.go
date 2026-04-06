package multipart

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"testing"
)

func createMultipartRequest(t *testing.T, fieldName string, files map[string][]byte, fields map[string]string) *http.Request {
	t.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)

	for k, v := range fields {
		w.WriteField(k, v)
	}

	for name, data := range files {
		part, err := w.CreateFormFile(fieldName, name)
		if err != nil {
			t.Fatal(err)
		}
		part.Write(data)
	}
	w.Close()

	req, err := http.NewRequest(http.MethodPost, "/upload", &buf)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", w.FormDataContentType())
	return req
}

func TestParse_SingleFile(t *testing.T) {
	req := createMultipartRequest(t, "file", map[string][]byte{
		"test.pdf": []byte("fake pdf content"),
	}, nil)

	cfg := DefaultConfig()
	cfg.AllowedTypes = nil // Don't check content type in this test (Go sets application/octet-stream).
	result, err := Parse(req, "file", cfg)
	if err != nil {
		t.Fatal(err)
	}

	if len(result.Files) != 1 {
		t.Fatalf("files: got %d", len(result.Files))
	}
	if result.Files[0].Filename != "test.pdf" {
		t.Errorf("filename: got %q", result.Files[0].Filename)
	}
	if result.Files[0].Extension != ".pdf" {
		t.Errorf("extension: got %q", result.Files[0].Extension)
	}
	if result.Files[0].Size != 16 {
		t.Errorf("size: got %d", result.Files[0].Size)
	}
}

func TestParse_WithFields(t *testing.T) {
	req := createMultipartRequest(t, "file", map[string][]byte{
		"doc.pdf": []byte("pdf"),
	}, map[string]string{
		"title":       "Test Document",
		"description": "A test",
	})

	cfg := DefaultConfig()
	cfg.AllowedTypes = nil // Don't check content type (Go default: octet-stream).
	result, err := Parse(req, "file", cfg)
	if err != nil {
		t.Fatal(err)
	}

	if result.Fields["title"] != "Test Document" {
		t.Errorf("title: got %q", result.Fields["title"])
	}
}

func TestParse_FileTooLarge(t *testing.T) {
	largeData := make([]byte, 1024*1024+1) // 1MB + 1 byte.
	req := createMultipartRequest(t, "file", map[string][]byte{
		"big.pdf": largeData,
	}, nil)

	cfg := Config{
		MaxFileSize:  1024 * 1024, // 1MB limit.
		MaxTotalSize: 50 * 1024 * 1024,
		AllowedExts:  map[string]bool{".pdf": true},
	}

	_, err := Parse(req, "file", cfg)
	if err == nil {
		t.Error("should reject file exceeding size limit")
	}
}

func TestParse_TooManyFiles(t *testing.T) {
	files := map[string][]byte{
		"a.pdf": []byte("a"),
		"b.pdf": []byte("b"),
		"c.pdf": []byte("c"),
	}
	req := createMultipartRequest(t, "file", files, nil)

	cfg := DefaultConfig()
	cfg.MaxFiles = 2

	_, err := Parse(req, "file", cfg)
	if err == nil {
		t.Error("should reject too many files")
	}
}

func TestParse_DisallowedExtension(t *testing.T) {
	req := createMultipartRequest(t, "file", map[string][]byte{
		"script.exe": []byte("malware"),
	}, nil)

	_, err := Parse(req, "file", DefaultConfig())
	if err == nil {
		t.Error("should reject disallowed extension")
	}
}

func TestValidateFilename(t *testing.T) {
	tests := []struct {
		name  string
		valid bool
	}{
		{"document.pdf", true},
		{"report-2024.pdf", true},
		{"", false},
		{"../../../etc/passwd", false},
		{"file/with/slash", false},
		{"file\\backslash", false},
		{"file<script>.pdf", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateFilename(tt.name)
			if (err == nil) != tt.valid {
				t.Errorf("ValidateFilename(%q): err=%v, want valid=%v", tt.name, err, tt.valid)
			}
		})
	}
}

func TestValidateFilename_TooLong(t *testing.T) {
	long := make([]byte, 256)
	for i := range long {
		long[i] = 'a'
	}
	if err := ValidateFilename(string(long)); err == nil {
		t.Error("filename > 255 chars should fail")
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.MaxFileSize != 10*1024*1024 {
		t.Errorf("max file size: got %d", cfg.MaxFileSize)
	}
	if cfg.MaxFiles != 5 {
		t.Errorf("max files: got %d", cfg.MaxFiles)
	}
	if !cfg.AllowedTypes["application/pdf"] {
		t.Error("should allow PDF")
	}
	if !cfg.AllowedExts[".pdf"] {
		t.Error("should allow .pdf extension")
	}
}
