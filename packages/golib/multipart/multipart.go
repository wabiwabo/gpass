// Package multipart provides safe multipart form handling with file
// upload validation, size limits, and content-type checking.
// Designed for document upload workflows in GarudaSign.
package multipart

import (
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"strings"
)

// Config defines upload constraints.
type Config struct {
	MaxFileSize   int64             // Max size per file in bytes.
	MaxTotalSize  int64             // Max total upload size.
	MaxFiles      int               // Max number of files.
	AllowedTypes  map[string]bool   // Allowed MIME types (e.g., "application/pdf").
	AllowedExts   map[string]bool   // Allowed extensions (e.g., ".pdf").
	RequiredField string            // Required file field name.
}

// DefaultConfig returns safe defaults for document uploads.
func DefaultConfig() Config {
	return Config{
		MaxFileSize:  10 * 1024 * 1024, // 10MB per file.
		MaxTotalSize: 50 * 1024 * 1024, // 50MB total.
		MaxFiles:     5,
		AllowedTypes: map[string]bool{
			"application/pdf":  true,
			"image/png":        true,
			"image/jpeg":       true,
			"application/xml":  true,
			"text/xml":         true,
		},
		AllowedExts: map[string]bool{
			".pdf":  true,
			".png":  true,
			".jpg":  true,
			".jpeg": true,
			".xml":  true,
		},
	}
}

// UploadedFile holds metadata and content of an uploaded file.
type UploadedFile struct {
	Filename    string `json:"filename"`
	Size        int64  `json:"size"`
	ContentType string `json:"content_type"`
	Extension   string `json:"extension"`
	Data        []byte `json:"-"` // Raw file data.
}

// Result holds the outcome of multipart parsing.
type Result struct {
	Files  []UploadedFile    `json:"files"`
	Fields map[string]string `json:"fields"`
	Total  int64             `json:"total_size"`
}

// Parse reads and validates a multipart request.
func Parse(r *http.Request, fieldName string, cfg Config) (*Result, error) {
	if cfg.MaxTotalSize <= 0 {
		cfg.MaxTotalSize = 50 * 1024 * 1024
	}

	if err := r.ParseMultipartForm(cfg.MaxTotalSize); err != nil {
		return nil, fmt.Errorf("multipart: parse form: %w", err)
	}
	defer r.MultipartForm.RemoveAll()

	result := &Result{
		Fields: make(map[string]string),
	}

	// Extract text fields.
	for k, v := range r.MultipartForm.Value {
		if len(v) > 0 {
			result.Fields[k] = v[0]
		}
	}

	// Extract files.
	files := r.MultipartForm.File[fieldName]
	if len(files) == 0 && cfg.RequiredField != "" {
		return nil, fmt.Errorf("multipart: no files in field %q", fieldName)
	}

	if cfg.MaxFiles > 0 && len(files) > cfg.MaxFiles {
		return nil, fmt.Errorf("multipart: too many files (%d), max %d", len(files), cfg.MaxFiles)
	}

	for _, fh := range files {
		uploaded, err := processFile(fh, cfg)
		if err != nil {
			return nil, err
		}
		result.Files = append(result.Files, *uploaded)
		result.Total += uploaded.Size
	}

	if cfg.MaxTotalSize > 0 && result.Total > cfg.MaxTotalSize {
		return nil, fmt.Errorf("multipart: total size %d exceeds limit %d", result.Total, cfg.MaxTotalSize)
	}

	return result, nil
}

func processFile(fh *multipart.FileHeader, cfg Config) (*UploadedFile, error) {
	// Size check.
	if cfg.MaxFileSize > 0 && fh.Size > cfg.MaxFileSize {
		return nil, fmt.Errorf("multipart: file %q size %d exceeds limit %d",
			fh.Filename, fh.Size, cfg.MaxFileSize)
	}

	// Extension check.
	ext := strings.ToLower(filepath.Ext(fh.Filename))
	if len(cfg.AllowedExts) > 0 && !cfg.AllowedExts[ext] {
		return nil, fmt.Errorf("multipart: file %q has disallowed extension %q",
			fh.Filename, ext)
	}

	// Sanitize filename (prevent path traversal).
	cleanName := filepath.Base(fh.Filename)
	if cleanName == "." || cleanName == "/" {
		return nil, fmt.Errorf("multipart: invalid filename")
	}

	// Read content.
	f, err := fh.Open()
	if err != nil {
		return nil, fmt.Errorf("multipart: open file: %w", err)
	}
	defer f.Close()

	data, err := io.ReadAll(io.LimitReader(f, cfg.MaxFileSize+1))
	if err != nil {
		return nil, fmt.Errorf("multipart: read file: %w", err)
	}

	if int64(len(data)) > cfg.MaxFileSize {
		return nil, fmt.Errorf("multipart: file %q exceeds size limit", cleanName)
	}

	// Content type from header.
	contentType := fh.Header.Get("Content-Type")
	if len(cfg.AllowedTypes) > 0 && contentType != "" {
		if !cfg.AllowedTypes[contentType] {
			return nil, fmt.Errorf("multipart: file %q has disallowed content type %q",
				cleanName, contentType)
		}
	}

	return &UploadedFile{
		Filename:    cleanName,
		Size:        int64(len(data)),
		ContentType: contentType,
		Extension:   ext,
		Data:        data,
	}, nil
}

// ValidateFilename checks for path traversal and dangerous characters.
func ValidateFilename(name string) error {
	if name == "" {
		return fmt.Errorf("empty filename")
	}
	if strings.Contains(name, "..") {
		return fmt.Errorf("path traversal detected")
	}
	if strings.ContainsAny(name, "/\\:*?\"<>|") {
		return fmt.Errorf("filename contains invalid characters")
	}
	if len(name) > 255 {
		return fmt.Errorf("filename too long")
	}
	return nil
}
