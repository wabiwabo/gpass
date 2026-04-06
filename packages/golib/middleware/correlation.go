package middleware

import (
	"context"
	"net/http"
)

const correlationKey contextKey = "correlation_ids"

// CorrelationIDs holds all tracing identifiers.
type CorrelationIDs struct {
	RequestID     string
	CorrelationID string
	TraceParent   string
}

// Correlation returns middleware that manages correlation IDs for distributed tracing.
// It handles three IDs:
//
//	X-Request-Id    — unique per request (generated if missing)
//	X-Correlation-Id — shared across a user session/transaction (generated if missing)
//	X-Trace-Parent  — W3C trace context (forwarded, not generated)
//
// All IDs are stored in context and set on the response.
func Correlation(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := r.Header.Get("X-Request-Id")
		if requestID == "" {
			requestID = generateUUID()
		}

		correlationID := r.Header.Get("X-Correlation-Id")
		if correlationID == "" {
			correlationID = generateUUID()
		}

		traceParent := r.Header.Get("X-Trace-Parent")

		ids := CorrelationIDs{
			RequestID:     requestID,
			CorrelationID: correlationID,
			TraceParent:   traceParent,
		}

		w.Header().Set("X-Request-Id", requestID)
		w.Header().Set("X-Correlation-Id", correlationID)
		if traceParent != "" {
			w.Header().Set("X-Trace-Parent", traceParent)
		}

		ctx := context.WithValue(r.Context(), correlationKey, ids)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// GetCorrelation retrieves correlation IDs from context.
func GetCorrelation(ctx context.Context) CorrelationIDs {
	if ctx == nil {
		return CorrelationIDs{}
	}
	if ids, ok := ctx.Value(correlationKey).(CorrelationIDs); ok {
		return ids
	}
	return CorrelationIDs{}
}

// PropagateHeaders adds correlation headers to an outgoing HTTP request.
func PropagateHeaders(req *http.Request, ids CorrelationIDs) {
	if ids.RequestID != "" {
		req.Header.Set("X-Request-Id", ids.RequestID)
	}
	if ids.CorrelationID != "" {
		req.Header.Set("X-Correlation-Id", ids.CorrelationID)
	}
	if ids.TraceParent != "" {
		req.Header.Set("X-Trace-Parent", ids.TraceParent)
	}
}
