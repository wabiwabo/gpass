package middleware

import (
	"bytes"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

// SensitiveHeaders lists header names whose values are redacted in audit logs.
var SensitiveHeaders = []string{
	"Authorization",
	"Cookie",
	"Set-Cookie",
	"X-API-Key",
	"X-Service-Signature",
	"BFF-Session-Secret",
}

// RedactHeaders returns a copy of headers with sensitive values replaced by "[REDACTED]".
func RedactHeaders(h http.Header) http.Header {
	redacted := h.Clone()
	for _, name := range SensitiveHeaders {
		if redacted.Get(name) != "" {
			redacted.Set(name, "[REDACTED]")
		}
	}
	return redacted
}

// auditResponseWriter captures the response body and status code for audit logging.
type auditResponseWriter struct {
	http.ResponseWriter
	status      int
	written     bool
	body        bytes.Buffer
	maxBodySize int
}

func (w *auditResponseWriter) WriteHeader(code int) {
	if !w.written {
		w.status = code
		w.written = true
	}
	w.ResponseWriter.WriteHeader(code)
}

func (w *auditResponseWriter) Write(b []byte) (int, error) {
	if !w.written {
		w.status = http.StatusOK
		w.written = true
	}
	// Capture response body up to maxBodySize.
	remaining := w.maxBodySize - w.body.Len()
	if remaining > 0 {
		toWrite := b
		if len(toWrite) > remaining {
			toWrite = toWrite[:remaining]
		}
		w.body.Write(toWrite)
	}
	return w.ResponseWriter.Write(b)
}

// AuditLog returns middleware that logs complete request and response details.
// Unlike AccessLog (which logs summary), this captures full bodies for compliance.
// WARNING: Enable only for specific endpoints — creates significant log volume.
//
// Captured: method, path, headers (redacted: Authorization, Cookie), request body,
// response status, response body (truncated to maxBodySize), latency, user_id.
func AuditLog(maxBodySize int) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			// Read and buffer the request body.
			var reqBody string
			if r.Body != nil {
				bodyBytes, err := io.ReadAll(r.Body)
				if err == nil {
					if len(bodyBytes) > maxBodySize {
						reqBody = string(bodyBytes[:maxBodySize])
					} else {
						reqBody = string(bodyBytes)
					}
					// Restore the body so the next handler can read it.
					r.Body = io.NopCloser(bytes.NewReader(bodyBytes))
				}
			}

			aw := &auditResponseWriter{
				ResponseWriter: w,
				status:         http.StatusOK,
				maxBodySize:    maxBodySize,
			}

			next.ServeHTTP(aw, r)

			duration := time.Since(start)
			userID := r.Header.Get("X-User-ID")

			// Build redacted header strings for logging.
			reqHeaders := RedactHeaders(r.Header)
			respHeaders := RedactHeaders(aw.Header())

			attrs := []any{
				slog.String("method", r.Method),
				slog.String("path", r.URL.Path),
				slog.Int("status", aw.status),
				slog.Duration("latency", duration),
				slog.String("request_headers", formatHeaders(reqHeaders)),
				slog.String("response_headers", formatHeaders(respHeaders)),
				slog.String("request_body", reqBody),
				slog.String("response_body", aw.body.String()),
			}
			if userID != "" {
				attrs = append(attrs, slog.String("user_id", userID))
			}

			slog.Info("audit", attrs...)
		})
	}
}

// formatHeaders converts http.Header into a compact string representation.
func formatHeaders(h http.Header) string {
	var b strings.Builder
	for k, vals := range h {
		for _, v := range vals {
			if b.Len() > 0 {
				b.WriteString("; ")
			}
			b.WriteString(k)
			b.WriteString("=")
			b.WriteString(v)
		}
	}
	return b.String()
}
