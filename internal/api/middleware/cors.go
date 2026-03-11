package middleware

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/madhavkobal/sangraha/internal/metadata"
)

// CORSRulesFetcher is a function that returns the CORS rules for a bucket.
// It is called on each request to look up live rules.
type CORSRulesFetcher func(bucket string) []metadata.CORSRule

// CORS returns middleware that applies per-bucket CORS rules. If fetcher is
// nil, the middleware is a no-op.
func CORS(fetcher CORSRulesFetcher) func(http.Handler) http.Handler {
	if fetcher == nil {
		return func(next http.Handler) http.Handler { return next }
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			if origin == "" {
				next.ServeHTTP(w, r)
				return
			}

			bucket := extractBucket(r)
			if bucket == "" {
				next.ServeHTTP(w, r)
				return
			}

			rules := fetcher(bucket)
			rule := matchCORSRule(rules, origin, r.Method)
			if rule == nil {
				// No matching rule — serve request without CORS headers.
				next.ServeHTTP(w, r)
				return
			}

			// Set CORS response headers.
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Vary", "Origin")
			if len(rule.AllowedMethods) > 0 {
				w.Header().Set("Access-Control-Allow-Methods", strings.Join(rule.AllowedMethods, ", "))
			}
			if len(rule.AllowedHeaders) > 0 {
				w.Header().Set("Access-Control-Allow-Headers", strings.Join(rule.AllowedHeaders, ", "))
			}
			if len(rule.ExposeHeaders) > 0 {
				w.Header().Set("Access-Control-Expose-Headers", strings.Join(rule.ExposeHeaders, ", "))
			}
			if rule.MaxAgeSeconds > 0 {
				w.Header().Set("Access-Control-Max-Age", strconv.Itoa(rule.MaxAgeSeconds))
			}

			// Pre-flight OPTIONS request.
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusOK)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// matchCORSRule finds the first rule that matches the given origin and method.
func matchCORSRule(rules []metadata.CORSRule, origin, method string) *metadata.CORSRule {
	for i := range rules {
		rule := &rules[i]
		if !originMatches(rule.AllowedOrigins, origin) {
			continue
		}
		if !methodMatches(rule.AllowedMethods, method) {
			continue
		}
		return rule
	}
	return nil
}

func originMatches(allowedOrigins []string, origin string) bool {
	for _, ao := range allowedOrigins {
		if ao == "*" || ao == origin {
			return true
		}
		// Simple wildcard: *.example.com
		if strings.HasPrefix(ao, "*.") {
			suffix := ao[1:] // ".example.com"
			if strings.HasSuffix(origin, suffix) {
				return true
			}
		}
	}
	return false
}

func methodMatches(allowedMethods []string, method string) bool {
	for _, am := range allowedMethods {
		if strings.EqualFold(am, method) || am == "*" {
			return true
		}
	}
	return false
}
