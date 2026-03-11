package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/madhavkobal/sangraha/internal/auth"
)

func TestRateLimitDisabledWhenZero(t *testing.T) {
	mw := RateLimit(0)
	called := 0
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called++
	}))

	for i := 0; i < 20; i++ {
		req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
	}
	if called != 20 {
		t.Errorf("disabled rate limiter: called %d times; want 20", called)
	}
}

func TestRateLimitBlocksExcessRequests(t *testing.T) {
	// Burst of 5 req/s; fire 15 back-to-back requests — at least some must be blocked.
	mw := RateLimit(5)
	allowed := 0
	blocked := 0
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		allowed++
	}))

	for i := 0; i < 15; i++ {
		req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
		req.RemoteAddr = "10.0.0.1:1234" // same IP to trigger per-IP limit
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		if rr.Code == http.StatusTooManyRequests {
			blocked++
		}
	}
	if blocked == 0 {
		t.Errorf("expected at least one blocked request with rps=5 and 15 requests, got 0")
	}
	t.Logf("allowed=%d blocked=%d", allowed, blocked)
}

func TestRateLimitPerKeyThrottling(t *testing.T) {
	mw := RateLimit(3)
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {}))

	// Inject a verified identity into context so the per-key limiter activates.
	identity := auth.VerifiedIdentity{AccessKey: "test-key"}

	allowed := 0
	blocked := 0
	for i := 0; i < 10; i++ {
		req := httptest.NewRequestWithContext(
			context.WithValue(context.Background(), identityContextKey{}, identity),
			http.MethodGet, "/", nil,
		)
		req.RemoteAddr = "10.0.0.2:5678"
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		if rr.Code == http.StatusTooManyRequests {
			blocked++
		} else {
			allowed++
		}
	}
	if blocked == 0 {
		t.Errorf("expected some per-key throttling with rps=3 and 10 requests, got 0 blocked")
	}
	t.Logf("allowed=%d blocked=%d", allowed, blocked)
}

func TestRateLimitDifferentIPsNotThrottledByEachOther(t *testing.T) {
	mw := RateLimit(5)
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {}))

	// Two different IPs each get their own bucket.
	for _, ip := range []string{"10.1.0.1:80", "10.2.0.2:80"} {
		req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
		req.RemoteAddr = ip
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		if rr.Code == http.StatusTooManyRequests {
			t.Errorf("first request from %s should not be throttled", ip)
		}
	}
}
