package admin

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHandleHealth(t *testing.T) {
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/admin/v1/health", nil)
	rr := httptest.NewRecorder()

	handleHealth(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("status = %d; want 200", rr.Code)
	}
	var resp healthResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Status != "ok" {
		t.Errorf("status = %q; want %q", resp.Status, "ok")
	}
}

func TestHandleReady(t *testing.T) {
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/admin/v1/ready", nil)
	rr := httptest.NewRecorder()

	handleReady(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("status = %d; want 200", rr.Code)
	}
	var resp readyResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Status != "ready" {
		t.Errorf("status = %q; want %q", resp.Status, "ready")
	}
}

func TestHandleInfo(t *testing.T) {
	handler := handleInfo("v1.0.0", "2026-01-01T00:00:00Z")
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/admin/v1/info", nil)
	rr := httptest.NewRecorder()

	handler(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("status = %d; want 200", rr.Code)
	}
	var resp infoResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Version != "v1.0.0" {
		t.Errorf("version = %q; want %q", resp.Version, "v1.0.0")
	}
	if resp.BuildTime != "2026-01-01T00:00:00Z" {
		t.Errorf("build_time = %q; want %q", resp.BuildTime, "2026-01-01T00:00:00Z")
	}
}

func TestWriteJSON(t *testing.T) {
	rr := httptest.NewRecorder()
	writeJSON(rr, http.StatusCreated, map[string]string{"key": "value"})

	if rr.Code != http.StatusCreated {
		t.Errorf("status = %d; want 201", rr.Code)
	}
	if ct := rr.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q; want application/json", ct)
	}
}

func TestMetricsEndpoint(t *testing.T) {
	handler := metricsHandler()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/admin/v1/metrics", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("metrics status = %d; want 200", rr.Code)
	}
	if ct := rr.Header().Get("Content-Type"); ct == "" {
		t.Error("metrics should return Content-Type header")
	}
}
