package httpx

import (
	"log/slog"
	"net/http"
	"time"
)

// statusRecorder captures the status code for access logging without
// disturbing the underlying ResponseWriter contract.
type statusRecorder struct {
	http.ResponseWriter
	status int
	bytes  int
}

func (s *statusRecorder) WriteHeader(code int) {
	s.status = code
	s.ResponseWriter.WriteHeader(code)
}

func (s *statusRecorder) Write(b []byte) (int, error) {
	if s.status == 0 {
		s.status = http.StatusOK
	}
	n, err := s.ResponseWriter.Write(b)
	s.bytes += n
	return n, err
}

// AccessLog emits one structured slog entry per request after it completes.
// Includes correlation ID (set by RequestID), method, path, status, bytes,
// duration, remote, and user-agent.
//
// Health/readiness probes are skipped to keep log volume sane.
func AccessLog(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Skip k8s probes — they hit /health and /readyz hundreds of
		// times per minute and would drown out real traffic in logs.
		if r.URL.Path == "/health" || r.URL.Path == "/readyz" {
			h.ServeHTTP(w, r)
			return
		}

		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w}
		h.ServeHTTP(rec, r)
		dur := time.Since(start)

		level := slog.LevelInfo
		if rec.status >= 500 {
			level = slog.LevelError
		} else if rec.status >= 400 {
			level = slog.LevelWarn
		}

		slog.LogAttrs(r.Context(), level, "http_request",
			slog.String("request_id", RequestIDFromContext(r.Context())),
			slog.String("method", r.Method),
			slog.String("path", r.URL.Path),
			slog.Int("status", rec.status),
			slog.Int("bytes", rec.bytes),
			slog.Duration("duration", dur),
			slog.String("remote", r.RemoteAddr),
			slog.String("user_agent", r.UserAgent()),
		)
	})
}
