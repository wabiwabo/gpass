package middleware

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/garudapass/gpass/apps/bff/session"
)

type sessionContextKey struct{}

// RequireSession ensures the request has a valid session cookie.
// If valid, the session data is stored in the request context for
// downstream handlers. If invalid or expired, responds with 401.
func RequireSession(store session.Store) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			cookie, err := r.Cookie("gpass_session")
			if err != nil {
				http.Error(w, `{"error":"unauthorized","message":"Authentication required"}`, http.StatusUnauthorized)
				return
			}

			data, err := store.Get(r.Context(), cookie.Value)
			if err != nil {
				slog.Warn("invalid session",
					"error", err,
					"request_id", GetRequestID(r.Context()),
				)
				http.Error(w, `{"error":"unauthorized","message":"Session invalid or expired"}`, http.StatusUnauthorized)
				return
			}

			// Check if session has expired (defense in depth — Redis TTL is primary)
			if !data.ExpiresAt.IsZero() && time.Now().After(data.ExpiresAt) {
				_ = store.Delete(r.Context(), cookie.Value)
				slog.Info("expired session cleaned up",
					"user_id", data.UserID,
					"request_id", GetRequestID(r.Context()),
				)
				http.Error(w, `{"error":"session_expired","message":"Session has expired, please login again"}`, http.StatusUnauthorized)
				return
			}

			ctx := context.WithValue(r.Context(), sessionContextKey{}, data)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// GetSessionData extracts session data from the request context.
// Returns nil if no session is present (e.g., unauthenticated route).
func GetSessionData(ctx context.Context) *session.Data {
	if data, ok := ctx.Value(sessionContextKey{}).(*session.Data); ok {
		return data
	}
	return nil
}
