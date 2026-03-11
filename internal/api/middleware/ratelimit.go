package middleware

import (
	"net/http"
	"sync"
	"time"
)

// tokenBucket implements a simple token-bucket rate limiter.
type tokenBucket struct {
	mu       sync.Mutex
	tokens   float64
	capacity float64
	rate     float64 // tokens per second
	lastFill time.Time
}

func newTokenBucket(rps float64) *tokenBucket {
	return &tokenBucket{
		tokens:   rps,
		capacity: rps,
		rate:     rps,
		lastFill: time.Now(),
	}
}

func (tb *tokenBucket) allow() bool {
	tb.mu.Lock()
	defer tb.mu.Unlock()
	now := time.Now()
	elapsed := now.Sub(tb.lastFill).Seconds()
	tb.lastFill = now
	tb.tokens += elapsed * tb.rate
	if tb.tokens > tb.capacity {
		tb.tokens = tb.capacity
	}
	if tb.tokens >= 1 {
		tb.tokens--
		return true
	}
	return false
}

// rateLimiter holds per-key token buckets.
type rateLimiter struct {
	mu      sync.Mutex
	buckets map[string]*tokenBucket
	rps     float64
}

func newRateLimiter(rps float64) *rateLimiter {
	return &rateLimiter{
		buckets: make(map[string]*tokenBucket),
		rps:     rps,
	}
}

func (rl *rateLimiter) allow(key string) bool {
	rl.mu.Lock()
	tb, ok := rl.buckets[key]
	if !ok {
		tb = newTokenBucket(rl.rps)
		rl.buckets[key] = tb
	}
	rl.mu.Unlock()
	return tb.allow()
}

// RateLimit returns middleware that limits requests per IP and per access key.
// rps is the requests-per-second allowed per client. Pass 0 to disable.
func RateLimit(rps int) func(http.Handler) http.Handler {
	if rps <= 0 {
		return func(next http.Handler) http.Handler { return next }
	}
	ipLimiter := newRateLimiter(float64(rps))
	keyLimiter := newRateLimiter(float64(rps))

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := realIP(r)
			if !ipLimiter.allow(ip) {
				http.Error(w, "rate limit exceeded", http.StatusTooManyRequests)
				return
			}

			identity, ok := IdentityFromContext(r.Context())
			if ok && identity.AccessKey != "" {
				if !keyLimiter.allow(identity.AccessKey) {
					http.Error(w, "rate limit exceeded", http.StatusTooManyRequests)
					return
				}
			}

			next.ServeHTTP(w, r)
		})
	}
}
