// Package transport exposes the ingest service over OTLP/gRPC and OTLP/HTTP.
package transport

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"google.golang.org/grpc/codes"

	"github.com/anushkagupta-06/eye-of-agamotto/services/ingestion-gateway/internal/batcher"
	"github.com/anushkagupta-06/eye-of-agamotto/services/ingestion-gateway/internal/ingest"
)

// Authenticator resolves an API key to a tenant id.
type Authenticator interface {
	Authenticate(key string) (tenant string, err error)
}

// RequestRecorder counts finished requests.
type RequestRecorder interface {
	Request(transport, signal string, code int)
}

type tenantKey struct{}

// WithTenant stores the authenticated tenant in ctx.
func WithTenant(ctx context.Context, tenant string) context.Context {
	return context.WithValue(ctx, tenantKey{}, tenant)
}

// TenantFrom returns the tenant stored by WithTenant.
func TenantFrom(ctx context.Context) string {
	t, _ := ctx.Value(tenantKey{}).(string)
	return t
}

// bearer extracts the token from an "Authorization: Bearer <token>" value.
func bearer(h string) string {
	const prefix = "bearer "
	if len(h) > len(prefix) && strings.EqualFold(h[:len(prefix)], prefix) {
		return strings.TrimSpace(h[len(prefix):])
	}
	return ""
}

// failure is a transport-neutral description of an error response. Messages are deliberately
// generic: internal error text goes to the logs, never to clients.
type failure struct {
	httpStatus int
	code       codes.Code
	message    string
	retry      bool // the client should retry after a delay
	log        bool // unexpected: log at error level
}

func classify(err error) failure {
	switch {
	case errors.Is(err, ingest.ErrRateLimited):
		return failure{http.StatusTooManyRequests, codes.ResourceExhausted, "rate limit exceeded", true, false}
	case errors.Is(err, batcher.ErrOverloaded):
		return failure{http.StatusTooManyRequests, codes.ResourceExhausted, "server overloaded, retry later", true, false}
	case errors.Is(err, batcher.ErrTooLarge):
		return failure{http.StatusRequestEntityTooLarge, codes.InvalidArgument, "request too large", false, false}
	case errors.Is(err, context.Canceled):
		return failure{499, codes.Canceled, "request canceled", false, false}
	case errors.Is(err, batcher.ErrUnavailable), errors.Is(err, batcher.ErrClosed), errors.Is(err, context.DeadlineExceeded):
		return failure{http.StatusServiceUnavailable, codes.Unavailable, "service unavailable, retry later", true, false}
	default:
		return failure{http.StatusServiceUnavailable, codes.Unavailable, "service unavailable, retry later", true, true}
	}
}

// retryAfterSeconds renders d for the Retry-After header, rounded up, minimum 1.
func retryAfterSeconds(d time.Duration) int {
	s := int((d + time.Second - 1) / time.Second)
	if s < 1 {
		return 1
	}
	return s
}
