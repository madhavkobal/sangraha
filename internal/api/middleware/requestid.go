// Package middleware provides reusable HTTP middleware for the sangraha API.
package middleware

import (
	"context"
	"net/http"

	"github.com/google/uuid"
)

type requestIDContextKey struct{}

// RequestID injects a unique request ID into the request context and sets the
// x-amz-request-id response header. S3 clients rely on this header for
// debugging.
func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get("x-amz-request-id")
		if id == "" {
			id = uuid.New().String()
		}
		ctx := context.WithValue(r.Context(), requestIDContextKey{}, id)
		w.Header().Set("x-amz-request-id", id)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// RequestIDFromContext returns the request ID stored by the RequestID middleware.
func RequestIDFromContext(ctx context.Context) string {
	v, _ := ctx.Value(requestIDContextKey{}).(string)
	return v
}
