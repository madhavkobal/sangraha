package admin

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

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
	handler := New(ks, nil, nil, "v1.0.0", "2026-01-01T00:00:00Z", "http://localhost:9000", &config.Config{})

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/admin/v1/health", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("health status = %d; want 200", rr.Code)
	}
}

func TestAdminRouterReady(t *testing.T) {
	ks := setupKeyStore(t)
	handler := New(ks, nil, nil, "v1.0.0", "2026-01-01T00:00:00Z", "http://localhost:9000", &config.Config{})

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/admin/v1/ready", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("ready status = %d; want 200", rr.Code)
	}
}

func TestAdminRouterInfo(t *testing.T) {
	ks := setupKeyStore(t)
	handler := New(ks, nil, nil, "v2.0.0", "2026-03-05T00:00:00Z", "http://localhost:9000", &config.Config{})

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/admin/v1/info", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("info status = %d; want 200", rr.Code)
	}
}

func TestAdminRouterMetrics(t *testing.T) {
	ks := setupKeyStore(t)
	handler := New(ks, nil, nil, "v1.0.0", "2026-01-01T00:00:00Z", "http://localhost:9000", &config.Config{})

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/admin/v1/metrics", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("metrics status = %d; want 200", rr.Code)
	}
}

func TestAdminRouterUsersUnauth(t *testing.T) {
	ks := setupKeyStore(t)
	handler := New(ks, nil, nil, "v1.0.0", "2026-01-01T00:00:00Z", "http://localhost:9000", &config.Config{})

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
	handler := New(ks, nil, nil, "v1.0.0", "2026-01-01T00:00:00Z", "http://localhost:9000", &config.Config{})

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
	handler := New(ks, nil, nil, "v1.0.0", "2026-01-01T00:00:00Z", "http://localhost:9000", &config.Config{})

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/admin/v1/connections", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code == http.StatusOK {
		t.Error("connections: expected auth required")
	}
}

func TestAdminRouterGCStatus(t *testing.T) {
	ks := setupKeyStore(t)
	handler := New(ks, nil, nil, "v1.0.0", "2026-01-01T00:00:00Z", "http://localhost:9000", &config.Config{})

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/admin/v1/gc/status", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code == http.StatusOK {
		t.Error("gc/status: expected auth required")
	}
}

func TestAdminRouterServerReload(t *testing.T) {
	ks := setupKeyStore(t)
	handler := New(ks, nil, nil, "v1.0.0", "2026-01-01T00:00:00Z", "http://localhost:9000", &config.Config{})

	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/admin/v1/server/reload", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code == http.StatusOK {
		t.Error("server/reload: expected auth required")
	}
}

func TestAdminRouterExportUnauth(t *testing.T) {
	ks := setupKeyStore(t)
	handler := New(ks, nil, nil, "v1.0.0", "2026-01-01T00:00:00Z", "http://localhost:9000", &config.Config{})

	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/admin/v1/export", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code == http.StatusOK {
		t.Error("export: expected auth required")
	}
}

func TestAdminRouterBackupScheduleUnauth(t *testing.T) {
	ks := setupKeyStore(t)
	handler := New(ks, nil, nil, "v1.0.0", "2026-01-01T00:00:00Z", "http://localhost:9000", &config.Config{})

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/admin/v1/backup/schedule", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code == http.StatusOK {
		t.Error("backup/schedule: expected auth required")
	}
}

func TestAdminRouterLogStream(t *testing.T) {
	ks := setupKeyStore(t)
	handler := New(ks, nil, nil, "v1.0.0", "2026-01-01T00:00:00Z", "http://localhost:9000", &config.Config{})

	// Use a real test server so we can cancel the long-lived SSE connection.
	srv := httptest.NewServer(handler)
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL+"/admin/v1/logs/stream", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	resp, doErr := http.DefaultClient.Do(req)
	if doErr != nil && ctx.Err() == nil {
		// Context-cancellation errors are expected — only fail on unexpected errors.
		t.Fatalf("do request: %v", doErr)
	}
	if resp != nil {
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Errorf("logs/stream: want 200 got %d", resp.StatusCode)
		}
	}
}
