package middleware

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"runtime/debug"

	apperrors "github.com/garudapass/gpass/packages/golib/errors"
)

// ProblemResponse represents an RFC 7807 Problem Details response.
type ProblemResponse struct {
	Type   string `json:"type,omitempty"`
	Title  string `json:"title"`
	Status int    `json:"status"`
	Detail string `json:"detail,omitempty"`
}

// ErrorHandler is a handler that may return an error.
type ErrorHandler func(w http.ResponseWriter, r *http.Request) error

// HandleError wraps an ErrorHandler and converts returned errors to
// structured RFC 7807 Problem Details responses.
func HandleError(fn ErrorHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		err := fn(w, r)
		if err == nil {
			return
		}

		// Check if it's an AppError.
		if appErr, ok := apperrors.IsAppError(err); ok {
			writeProblem(w, r, ProblemResponse{
				Type:   "about:blank",
				Title:  appErr.Code,
				Status: appErr.HTTPStatus,
				Detail: appErr.Message,
			})
			return
		}

		// Generic internal error.
		slog.Error("unhandled error",
			"error", err,
			"path", r.URL.Path,
			"method", r.Method,
			"request_id", r.Header.Get("X-Request-Id"),
		)
		writeProblem(w, r, ProblemResponse{
			Type:   "about:blank",
			Title:  "internal_error",
			Status: http.StatusInternalServerError,
			Detail: "An internal error occurred",
		})
	}
}

// PanicRecovery returns middleware that recovers from panics and returns
// a structured 500 response instead of crashing the process.
// Unlike the basic Recovery middleware, this produces RFC 7807 responses
// and captures the stack trace for logging.
func PanicRecovery(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				stack := string(debug.Stack())
				slog.Error("panic recovered",
					"panic", rec,
					"path", r.URL.Path,
					"method", r.Method,
					"request_id", r.Header.Get("X-Request-Id"),
					"stack", stack,
				)

				writeProblem(w, r, ProblemResponse{
					Type:   "about:blank",
					Title:  "internal_error",
					Status: http.StatusInternalServerError,
					Detail: "An unexpected error occurred",
				})
			}
		}()

		next.ServeHTTP(w, r)
	})
}

func writeProblem(w http.ResponseWriter, _ *http.Request, problem ProblemResponse) {
	w.Header().Set("Content-Type", "application/problem+json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(problem.Status)
	json.NewEncoder(w).Encode(problem)
}
