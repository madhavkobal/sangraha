package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/madhavkobal/sangraha/internal/audit"
)

func TestAuditMiddleware(t *testing.T) {
	// Discard audit logger (empty path).
	logger, err := audit.New("")
	if err != nil {
		t.Fatalf("audit.New: %v", err)
	}

	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := Audit(logger)(next)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodPut, "/my-bucket/my-key", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("status = %d; want 200", rr.Code)
	}
}

func TestAuditMiddlewareCaptures4xx(t *testing.T) {
	logger, _ := audit.New("")
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})
	handler := Audit(logger)(next)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/bucket/key", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("status = %d; want 404", rr.Code)
	}
}

func TestAuditMiddlewareNilLogger(t *testing.T) {
	// Should not panic when logger is nil.
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := Audit(nil)(next)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodDelete, "/b/k", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("status = %d; want 200", rr.Code)
	}
}
