package s3

import (
	"context"

	"github.com/madhavkobal/sangraha/internal/api/middleware"
	"github.com/madhavkobal/sangraha/internal/auth"
)

type requestIDKey struct{}

// identityFromContext retrieves the verified identity set by the auth middleware.
// Returns a zero-value identity if none is present (should not happen in production).
func identityFromContext(ctx context.Context) auth.VerifiedIdentity {
	id, _ := middleware.IdentityFromContext(ctx)
	return id
}

// requestIDFromContext returns the request ID from the context.
func requestIDFromContext(ctx context.Context) string {
	v, _ := ctx.Value(requestIDKey{}).(string)
	return v
}

// withRequestID stores the request ID in the context.
func withRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, requestIDKey{}, id)
}

// withIdentity stores a verified identity in the context. Used in tests.
func withIdentity(ctx context.Context, id auth.VerifiedIdentity) context.Context {
	return middleware.SetIdentityInContext(ctx, id)
}
