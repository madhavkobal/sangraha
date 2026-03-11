package admin

import (
	"context"
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
	handler := New(ks, nil, nil, "v1.0.0", "2026-01-01T00:00:00Z", "http://localhost:9000", &config.Config{}, nil)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/admin/v1/health", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("health status = %d; want 200", rr.Code)
	}
}

func TestAdminRouterReady(t *testing.T) {
	ks := setupKeyStore(t)
	handler := New(ks, nil, nil, "v1.0.0", "2026-01-01T00:00:00Z", "http://localhost:9000", &config.Config{}, nil)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/admin/v1/ready", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("ready status = %d; want 200", rr.Code)
	}
}

func TestAdminRouterInfo(t *testing.T) {
	ks := setupKeyStore(t)
	handler := New(ks, nil, nil, "v2.0.0", "2026-03-05T00:00:00Z", "http://localhost:9000", &config.Config{}, nil)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/admin/v1/info", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("info status = %d; want 200", rr.Code)
	}
}

func TestAdminRouterMetrics(t *testing.T) {
	ks := setupKeyStore(t)
	handler := New(ks, nil, nil, "v1.0.0", "2026-01-01T00:00:00Z", "http://localhost:9000", &config.Config{}, nil)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/admin/v1/metrics", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("metrics status = %d; want 200", rr.Code)
	}
}

func TestAdminRouterUsersUnauth(t *testing.T) {
	ks := setupKeyStore(t)
	handler := New(ks, nil, nil, "v1.0.0", "2026-01-01T00:00:00Z", "http://localhost:9000", &config.Config{}, nil)

	// Unauthenticated request to protected endpoint.
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/admin/v1/users", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code == http.StatusOK {
		t.Error("expected non-200 for unauthenticated users request")
	}
}

func TestAdminRouterTLSInfo(t *testing.T) {
	ks := setupKeyStore(t)
	handler := New(ks, nil, nil, "v1.0.0", "2026-01-01T00:00:00Z", "http://localhost:9000", &config.Config{}, nil)

	// TLS info is auth-protected; unauthenticated must not return 200.
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/admin/v1/tls", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code == http.StatusOK {
		t.Error("TLS info: expected non-200 for unauthenticated request")
	}
}

func TestAdminRouterConnections(t *testing.T) {
	ks := setupKeyStore(t)
	handler := New(ks, nil, nil, "v1.0.0", "2026-01-01T00:00:00Z", "http://localhost:9000", &config.Config{}, nil)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/admin/v1/connections", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code == http.StatusOK {
		t.Error("connections: expected auth required")
	}
}

func TestAdminRouterGCStatus(t *testing.T) {
	ks := setupKeyStore(t)
	handler := New(ks, nil, nil, "v1.0.0", "2026-01-01T00:00:00Z", "http://localhost:9000", &config.Config{}, nil)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/admin/v1/gc/status", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code == http.StatusOK {
		t.Error("gc/status: expected auth required")
	}
}

func TestAdminRouterServerReload(t *testing.T) {
	ks := setupKeyStore(t)
	handler := New(ks, nil, nil, "v1.0.0", "2026-01-01T00:00:00Z", "http://localhost:9000", &config.Config{}, nil)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/admin/v1/server/reload", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code == http.StatusOK {
		t.Error("server/reload: expected auth required")
	}
}

func TestAdminRouterExportUnauth(t *testing.T) {
	ks := setupKeyStore(t)
	handler := New(ks, nil, nil, "v1.0.0", "2026-01-01T00:00:00Z", "http://localhost:9000", &config.Config{}, nil)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/admin/v1/export", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code == http.StatusOK {
		t.Error("export: expected auth required")
	}
}

func TestAdminRouterBackupScheduleUnauth(t *testing.T) {
	ks := setupKeyStore(t)
	handler := New(ks, nil, nil, "v1.0.0", "2026-01-01T00:00:00Z", "http://localhost:9000", &config.Config{}, nil)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/admin/v1/backup/schedule", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code == http.StatusOK {
		t.Error("backup/schedule: expected auth required")
	}
}

func TestAdminRouterLogStream(t *testing.T) {
	ks := setupKeyStore(t)
	handler := New(ks, nil, nil, "v1.0.0", "2026-01-01T00:00:00Z", "http://localhost:9000", &config.Config{}, nil)

	// Use a pre-cancelled context so the SSE handler exits immediately after
	// writing the initial 200 + "connected" comment, avoiding any blocking.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	req := httptest.NewRequestWithContext(ctx, http.MethodGet, "/admin/v1/logs/stream", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("logs/stream: want 200 got %d", rr.Code)
	}
}
