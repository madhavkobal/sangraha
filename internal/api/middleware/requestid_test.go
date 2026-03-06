package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRequestID(t *testing.T) {
	var capturedCtxID string
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedCtxID = RequestIDFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	})

	handler := RequestID(next)
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if capturedCtxID == "" {
		t.Error("request ID should be set in context")
	}
	if rr.Header().Get("x-amz-request-id") == "" {
		t.Error("x-amz-request-id response header should be set")
	}
	if rr.Header().Get("x-amz-request-id") != capturedCtxID {
		t.Errorf("response header ID %q != context ID %q", rr.Header().Get("x-amz-request-id"), capturedCtxID)
	}
}

func TestRequestIDPreservesClientID(t *testing.T) {
	// If the client provides an x-amz-request-id, preserve it.
	var capturedID string
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedID = RequestIDFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	})
	handler := RequestID(next)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	req.Header.Set("x-amz-request-id", "client-provided-id")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if capturedID != "client-provided-id" {
		t.Errorf("capturedID = %q; want %q", capturedID, "client-provided-id")
	}
}

func TestRequestIDUniqueness(t *testing.T) {
	seen := map[string]struct{}{}
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := RequestIDFromContext(r.Context())
		if _, dup := seen[id]; dup {
			t.Errorf("duplicate request ID: %q", id)
		}
		seen[id] = struct{}{}
		w.WriteHeader(http.StatusOK)
	})

	handler := RequestID(next)
	for i := 0; i < 10; i++ {
		req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
	}
}
