package multipart

import (
	"bytes"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// buildMultipart returns a request whose body is a multipart/form-data
// payload with the given files (filename → content-type → bytes) and
// optional text fields.
func buildMultipart(t *testing.T, fieldName string, files []fileSpec, fields map[string]string) *http.Request {
	t.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	for _, f := range files {
		hdr := make(map[string][]string)
		hdr["Content-Disposition"] = []string{
			`form-data; name="` + fieldName + `"; filename="` + f.filename + `"`,
		}
		if f.contentType != "" {
			hdr["Content-Type"] = []string{f.contentType}
		}
		fw, err := w.CreatePart(hdr)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := io.Copy(fw, bytes.NewReader(f.body)); err != nil {
			t.Fatal(err)
		}
	}
	for k, v := range fields {
		if err := w.WriteField(k, v); err != nil {
			t.Fatal(err)
		}
	}
	w.Close()
	req := httptest.NewRequest(http.MethodPost, "/upload", &buf)
	req.Header.Set("Content-Type", w.FormDataContentType())
	return req
}

type fileSpec struct {
	filename    string
	contentType string
	body        []byte
}

// TestParse_HappyPath_FilesAndFields covers the canonical successful
// upload: one PDF file, one text field, all under the limits.
func TestParse_HappyPath_FilesAndFields(t *testing.T) {
	req := buildMultipart(t, "doc",
		[]fileSpec{{filename: "contract.pdf", contentType: "application/pdf", body: []byte("%PDF-1.4 fake")}},
		map[string]string{"signer": "alice"},
	)
	cfg := DefaultConfig()
	res, err := Parse(req, "doc", cfg)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(res.Files) != 1 {
		t.Fatalf("got %d files, want 1", len(res.Files))
	}
	f := res.Files[0]
	if f.Filename != "contract.pdf" || f.Extension != ".pdf" {
		t.Errorf("file metadata: %+v", f)
	}
	if res.Fields["signer"] != "alice" {
		t.Errorf("text field lost: %v", res.Fields)
	}
	if res.Total != int64(len("%PDF-1.4 fake")) {
		t.Errorf("Total = %d", res.Total)
	}
}

// TestParse_TooManyFiles_Cov covers the MaxFiles enforcement branch.
func TestParse_TooManyFiles_Cov(t *testing.T) {
	files := []fileSpec{
		{"a.pdf", "application/pdf", []byte("a")},
		{"b.pdf", "application/pdf", []byte("b")},
		{"c.pdf", "application/pdf", []byte("c")},
	}
	req := buildMultipart(t, "doc", files, nil)
	cfg := DefaultConfig()
	cfg.MaxFiles = 2
	_, err := Parse(req, "doc", cfg)
	if err == nil || !strings.Contains(err.Error(), "too many files (3), max 2") {
		t.Errorf("err = %v", err)
	}
}

// TestParse_RequiredFieldMissing covers the empty-files-with-required
// branch.
func TestParse_RequiredFieldMissing(t *testing.T) {
	req := buildMultipart(t, "doc", nil, map[string]string{"signer": "alice"})
	cfg := DefaultConfig()
	cfg.RequiredField = "doc"
	_, err := Parse(req, "doc", cfg)
	if err == nil || !strings.Contains(err.Error(), `no files in field "doc"`) {
		t.Errorf("err = %v", err)
	}
}

// TestProcessFile_DisallowedExtension covers the extension allow-list
// rejection branch.
func TestProcessFile_DisallowedExtension(t *testing.T) {
	req := buildMultipart(t, "doc",
		[]fileSpec{{filename: "evil.exe", contentType: "application/octet-stream", body: []byte("MZ")}},
		nil,
	)
	cfg := DefaultConfig()
	_, err := Parse(req, "doc", cfg)
	if err == nil || !strings.Contains(err.Error(), `disallowed extension ".exe"`) {
		t.Errorf("err = %v", err)
	}
}

// TestProcessFile_DisallowedContentType covers the MIME allow-list
// rejection branch (extension allowed but Content-Type isn't).
func TestProcessFile_DisallowedContentType(t *testing.T) {
	req := buildMultipart(t, "doc",
		[]fileSpec{{filename: "doc.pdf", contentType: "application/x-msdownload", body: []byte("MZ")}},
		nil,
	)
	cfg := DefaultConfig()
	_, err := Parse(req, "doc", cfg)
	if err == nil || !strings.Contains(err.Error(), "disallowed content type") {
		t.Errorf("err = %v", err)
	}
}

// TestProcessFile_OversizedFile covers the size-limit branch.
func TestProcessFile_OversizedFile(t *testing.T) {
	big := bytes.Repeat([]byte("A"), 4096)
	req := buildMultipart(t, "doc",
		[]fileSpec{{filename: "big.pdf", contentType: "application/pdf", body: big}},
		nil,
	)
	cfg := DefaultConfig()
	cfg.MaxFileSize = 100
	_, err := Parse(req, "doc", cfg)
	if err == nil {
		t.Fatal("expected size error")
	}
	// Either of the size-rejection branches is acceptable; both pin
	// the same security guarantee.
	msg := err.Error()
	if !strings.Contains(msg, "exceeds") {
		t.Errorf("err = %q", msg)
	}
}

// TestValidateFilename_AllBranches covers each rejection rule and the
// happy path.
func TestValidateFilename_AllBranches(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string // substring of expected err, "" for success
	}{
		{"empty", "", "empty filename"},
		{"path traversal", "../etc/passwd", "path traversal"},
		{"slash", "a/b.pdf", "invalid characters"},
		{"backslash", "a\\b.pdf", "invalid characters"},
		{"too long", strings.Repeat("a", 256) + ".pdf", "too long"},
		{"valid", "contract.pdf", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateFilename(tc.in)
			if tc.want == "" {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Errorf("err = %v, want substring %q", err, tc.want)
			}
		})
	}
}
