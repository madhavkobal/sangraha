package admin

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/madhavkobal/sangraha/internal/auth"
	"github.com/madhavkobal/sangraha/internal/config"
	bboltstore "github.com/madhavkobal/sangraha/internal/metadata/bbolt"
)

func setupKeyStore(t *testing.T) *auth.KeyStore {
	t.Helper()
	s, err := bboltstore.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return auth.NewKeyStore(s)
}

func TestAdminRouterHealth(t *testing.T) {
	ks := setupKeyStore(t)
	handler := New(ks, "v1.0.0", "2026-01-01T00:00:00Z", "http://localhost:9000", &config.Config{})

	req := httptest.NewRequest(http.MethodGet, "/admin/v1/health", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("health status = %d; want 200", rr.Code)
	}
}

func TestAdminRouterReady(t *testing.T) {
	ks := setupKeyStore(t)
	handler := New(ks, "v1.0.0", "2026-01-01T00:00:00Z", "http://localhost:9000", &config.Config{})

	req := httptest.NewRequest(http.MethodGet, "/admin/v1/ready", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("ready status = %d; want 200", rr.Code)
	}
}

func TestAdminRouterInfo(t *testing.T) {
	ks := setupKeyStore(t)
	handler := New(ks, "v2.0.0", "2026-03-05T00:00:00Z", "http://localhost:9000", &config.Config{})

	req := httptest.NewRequest(http.MethodGet, "/admin/v1/info", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("info status = %d; want 200", rr.Code)
	}
}

func TestAdminRouterMetrics(t *testing.T) {
	ks := setupKeyStore(t)
	handler := New(ks, "v1.0.0", "2026-01-01T00:00:00Z", "http://localhost:9000", &config.Config{})

	req := httptest.NewRequest(http.MethodGet, "/admin/v1/metrics", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("metrics status = %d; want 200", rr.Code)
	}
}

func TestAdminRouterUsersUnauth(t *testing.T) {
	ks := setupKeyStore(t)
	handler := New(ks, "v1.0.0", "2026-01-01T00:00:00Z", "http://localhost:9000", &config.Config{})

	// Unauthenticated request to protected endpoint.
	req := httptest.NewRequest(http.MethodGet, "/admin/v1/users", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code == http.StatusOK {
		t.Error("expected non-200 for unauthenticated users request")
	}
}
