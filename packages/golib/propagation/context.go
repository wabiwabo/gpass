package propagation

import (
	"context"
	"net/http"
)

// Headers defines the standard propagation headers.
const (
	HeaderUserID        = "X-User-ID"
	HeaderRequestID     = "X-Request-Id"
	HeaderCorrelationID = "X-Correlation-Id"
	HeaderTraceParent   = "Traceparent"
	HeaderAuthMethod    = "X-Auth-Method"
	HeaderAuthSubject   = "X-Auth-Subject"
	HeaderServiceName   = "X-Source-Service"
	HeaderSessionID     = "X-Session-ID"
)

// contextKey is an unexported type for context keys in this package.
type contextKey struct{}

// ServiceContext holds all propagatable context for inter-service calls.
type ServiceContext struct {
	UserID        string
	RequestID     string
	CorrelationID string
	TraceParent   string
	AuthMethod    string
	AuthSubject   string
	SourceService string
	SessionID     string
}

// Extract extracts ServiceContext from an incoming HTTP request.
func Extract(r *http.Request) ServiceContext {
	return ServiceContext{
		UserID:        r.Header.Get(HeaderUserID),
		RequestID:     r.Header.Get(HeaderRequestID),
		CorrelationID: r.Header.Get(HeaderCorrelationID),
		TraceParent:   r.Header.Get(HeaderTraceParent),
		AuthMethod:    r.Header.Get(HeaderAuthMethod),
		AuthSubject:   r.Header.Get(HeaderAuthSubject),
		SourceService: r.Header.Get(HeaderServiceName),
		SessionID:     r.Header.Get(HeaderSessionID),
	}
}

// Inject injects ServiceContext into an outgoing HTTP request.
func Inject(req *http.Request, sc ServiceContext) {
	set := func(key, val string) {
		if val != "" {
			req.Header.Set(key, val)
		}
	}
	set(HeaderUserID, sc.UserID)
	set(HeaderRequestID, sc.RequestID)
	set(HeaderCorrelationID, sc.CorrelationID)
	set(HeaderTraceParent, sc.TraceParent)
	set(HeaderAuthMethod, sc.AuthMethod)
	set(HeaderAuthSubject, sc.AuthSubject)
	set(HeaderServiceName, sc.SourceService)
	set(HeaderSessionID, sc.SessionID)
}

// FromContext extracts ServiceContext stored in context.Context.
// Returns an empty ServiceContext if none is stored.
func FromContext(ctx context.Context) ServiceContext {
	sc, _ := ctx.Value(contextKey{}).(ServiceContext)
	return sc
}

// ToContext stores ServiceContext in context.Context.
func ToContext(ctx context.Context, sc ServiceContext) context.Context {
	return context.WithValue(ctx, contextKey{}, sc)
}

// Middleware extracts propagation headers from the incoming request
// and stores them in the request context for downstream use.
func Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sc := Extract(r)
		ctx := ToContext(r.Context(), sc)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// PropagatingClient wraps http.Client to automatically propagate context
// headers from the request context into outgoing HTTP requests.
type PropagatingClient struct {
	client  *http.Client
	service string
}

// NewPropagatingClient creates a new PropagatingClient that injects the given
// service name as the source service header on all outgoing requests.
func NewPropagatingClient(client *http.Client, serviceName string) *PropagatingClient {
	return &PropagatingClient{
		client:  client,
		service: serviceName,
	}
}

// Do executes the request with propagated context from ctx.
// It extracts the ServiceContext from ctx, overrides SourceService with
// the client's configured service name, and injects all headers.
func (c *PropagatingClient) Do(ctx context.Context, req *http.Request) (*http.Response, error) {
	sc := FromContext(ctx)
	sc.SourceService = c.service
	Inject(req, sc)
	return c.client.Do(req.WithContext(ctx))
}
