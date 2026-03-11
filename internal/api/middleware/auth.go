package middleware

import (
	"context"
	"net/http"
	"time"

	"github.com/madhavkobal/sangraha/internal/auth"
)

type identityContextKey struct{}

// Auth returns middleware that verifies AWS SigV4 requests. Unauthenticated
// requests are rejected with 403 AccessDenied.
func Auth(keyStore *auth.KeyStore) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Allow presigned URL requests that carry auth in query params.
			if isPresigned(r) {
				handlePresigned(w, r, next, keyStore)
				return
			}

			accessKey := auth.ExtractAccessKey(r)
			if accessKey == "" {
				writeAuthError(w, "AccessDenied", "missing Authorization header")
				return
			}

			rec, err := keyStore.Lookup(r.Context(), accessKey)
			if err != nil {
				writeAuthError(w, "InvalidAccessKeyId", "access key not found")
				return
			}

			// Full SigV4 signature verification using the stored signing key.
			if rec.SigningKey != "" {
				if verr := auth.VerifyRequest(r, rec.SigningKey, time.Now().UTC()); verr != nil {
					writeAuthError(w, "SignatureDoesNotMatch", verr.Error())
					return
				}
			}

			identity := auth.VerifiedIdentity{
				AccessKey: rec.AccessKey,
				Owner:     rec.Owner,
				IsRoot:    rec.IsRoot,
			}
			ctx := context.WithValue(r.Context(), identityContextKey{}, identity)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// isPresigned returns true when the request carries SigV4 presigned params.
func isPresigned(r *http.Request) bool {
	q := r.URL.Query()
	return q.Get("X-Amz-Signature") != "" || q.Get("x-amz-signature") != ""
}

// handlePresigned verifies a presigned URL request.
func handlePresigned(w http.ResponseWriter, r *http.Request, next http.Handler, keyStore *auth.KeyStore) {
	q := r.URL.Query()
	credParam := q.Get("X-Amz-Credential")
	if credParam == "" {
		credParam = q.Get("x-amz-credential")
	}
	// Credential is accessKey/date/region/service/aws4_request — extract key.
	accessKey := credParam
	for i, c := range credParam {
		if c == '/' {
			accessKey = credParam[:i]
			break
		}
	}
	if accessKey == "" {
		writeAuthError(w, "AccessDenied", "missing presign credential")
		return
	}

	rec, err := keyStore.Lookup(r.Context(), accessKey)
	if err != nil {
		writeAuthError(w, "InvalidAccessKeyId", "access key not found")
		return
	}

	if verr := auth.VerifyPresignedURL(r, rec.SigningKey, time.Now().UTC()); verr != nil {
		writeAuthError(w, "SignatureDoesNotMatch", verr.Error())
		return
	}

	identity := auth.VerifiedIdentity{
		AccessKey: rec.AccessKey,
		Owner:     rec.Owner,
		IsRoot:    rec.IsRoot,
	}
	ctx := context.WithValue(r.Context(), identityContextKey{}, identity)
	next.ServeHTTP(w, r.WithContext(ctx))
}

// IdentityFromContext retrieves the authenticated identity from the context.
func IdentityFromContext(ctx context.Context) (auth.VerifiedIdentity, bool) {
	v, ok := ctx.Value(identityContextKey{}).(auth.VerifiedIdentity)
	return v, ok
}

// SetIdentityInContext stores an identity in the context. Used by tests and
// other packages that need to inject an identity without running the full auth middleware.
func SetIdentityInContext(ctx context.Context, id auth.VerifiedIdentity) context.Context {
	return context.WithValue(ctx, identityContextKey{}, id)
}

func writeAuthError(w http.ResponseWriter, code, msg string) {
	w.Header().Set("Content-Type", "application/xml")
	w.WriteHeader(http.StatusForbidden)
	_, _ = w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?><Error><Code>` + code + `</Code><Message>` + msg + `</Message></Error>`)) //nolint:gosec // G705: code and msg are internal constants; Content-Type is application/xml
}
