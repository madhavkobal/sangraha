package middleware

import (
	"net/http"
	"strings"
	"time"

	"github.com/madhavkobal/sangraha/internal/audit"
)

// statusCapture wraps ResponseWriter to capture the status code and bytes.
type statusCapture struct {
	http.ResponseWriter
	code  int
	bytes int64
}

// WriteHeader captures the status code and delegates to the underlying writer.
func (sc *statusCapture) WriteHeader(code int) {
	sc.code = code
	sc.ResponseWriter.WriteHeader(code)
}

// Write captures the byte count and delegates to the underlying writer.
func (sc *statusCapture) Write(b []byte) (int, error) {
	n, err := sc.ResponseWriter.Write(b)
	sc.bytes += int64(n)
	return n, err
}

// Audit returns middleware that emits a structured audit event after each
// request.
func Audit(logger *audit.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			sc := &statusCapture{ResponseWriter: w, code: http.StatusOK}

			next.ServeHTTP(sc, r)

			identity, _ := IdentityFromContext(r.Context())
			event := audit.Event{
				RequestID:  RequestIDFromContext(r.Context()),
				User:       identity.AccessKey,
				Action:     inferAction(r),
				Bucket:     extractBucket(r),
				Key:        extractKey(r),
				SourceIP:   realIP(r),
				StatusCode: sc.code,
				Bytes:      sc.bytes,
				DurationMS: time.Since(start).Milliseconds(),
			}
			logger.Log(event)
		})
	}
}

func inferAction(r *http.Request) string {
	switch r.Method {
	case http.MethodGet:
		return "s3:GetObject"
	case http.MethodPut:
		return "s3:PutObject"
	case http.MethodDelete:
		return "s3:DeleteObject"
	case http.MethodHead:
		return "s3:HeadObject"
	case http.MethodPost:
		return "s3:PostObject"
	default:
		return r.Method
	}
}

func extractBucket(r *http.Request) string {
	parts := strings.SplitN(strings.TrimPrefix(r.URL.Path, "/"), "/", 2)
	if len(parts) > 0 {
		return parts[0]
	}
	return ""
}

func extractKey(r *http.Request) string {
	parts := strings.SplitN(strings.TrimPrefix(r.URL.Path, "/"), "/", 2)
	if len(parts) == 2 {
		return parts[1]
	}
	return ""
}

func realIP(r *http.Request) string {
	if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
		return strings.SplitN(fwd, ",", 2)[0]
	}
	if rip := r.Header.Get("X-Real-Ip"); rip != "" {
		return rip
	}
	return r.RemoteAddr
}
