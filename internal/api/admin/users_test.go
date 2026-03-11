package admin

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/madhavkobal/sangraha/internal/auth"
	bboltstore "github.com/madhavkobal/sangraha/internal/metadata/bbolt"
)

func setupUserHandler(t *testing.T) *userHandler {
	t.Helper()
	s, err := bboltstore.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return &userHandler{keyStore: auth.NewKeyStore(s)}
}

func TestCreateUser(t *testing.T) {
	h := setupUserHandler(t)

	body, _ := json.Marshal(map[string]string{"owner": "alice"})
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/admin/v1/users", bytes.NewReader(body))
	rr := httptest.NewRecorder()
	h.create(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("status = %d; want 201; body: %s", rr.Code, rr.Body.String())
	}
	var resp createUserResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.AccessKey == "" || resp.SecretKey == "" {
		t.Error("access key and secret key must be set")
	}
	if resp.Owner != "alice" {
		t.Errorf("owner = %q; want %q", resp.Owner, "alice")
	}
}

func TestCreateUserMissingOwner(t *testing.T) {
	h := setupUserHandler(t)

	body, _ := json.Marshal(map[string]string{"owner": ""})
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/admin/v1/users", bytes.NewReader(body))
	rr := httptest.NewRecorder()
	h.create(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("status = %d; want 400", rr.Code)
	}
}

func TestCreateUserInvalidJSON(t *testing.T) {
	h := setupUserHandler(t)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/admin/v1/users", bytes.NewReader([]byte("not json")))
	rr := httptest.NewRecorder()
	h.create(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("status = %d; want 400", rr.Code)
	}
}

func TestListUsers(t *testing.T) {
	h := setupUserHandler(t)
	ctx := context.Background()

	_, _, _ = h.keyStore.CreateKey(ctx, "user1", false)
	_, _, _ = h.keyStore.CreateKey(ctx, "user2", false)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/admin/v1/users", nil)
	rr := httptest.NewRecorder()
	h.list(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d; want 200", rr.Code)
	}
	var resp []map[string]interface{}
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(resp) != 2 {
		t.Errorf("got %d users; want 2", len(resp))
	}
}

func TestDeleteUser(t *testing.T) {
	h := setupUserHandler(t)
	ctx := context.Background()

	ak, _, _ := h.keyStore.CreateKey(ctx, "bob", false)

	// Build chi request context with URL param.
	chiCtx := chi.NewRouteContext()
	chiCtx.URLParams.Add("accessKey", ak)
	req := httptest.NewRequestWithContext(context.Background(), http.MethodDelete, "/admin/v1/users/"+ak, nil)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, chiCtx))
	rr := httptest.NewRecorder()
	h.delete(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Errorf("status = %d; want 204; body: %s", rr.Code, rr.Body.String())
	}
}

func TestRotateKey(t *testing.T) {
	h := setupUserHandler(t)
	ctx := context.Background()

	ak, _, _ := h.keyStore.CreateKey(ctx, "charlie", false)

	chiCtx := chi.NewRouteContext()
	chiCtx.URLParams.Add("accessKey", ak)
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/admin/v1/users/"+ak+"/keys/rotate", nil)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, chiCtx))
	rr := httptest.NewRecorder()
	h.rotateKey(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d; want 200; body: %s", rr.Code, rr.Body.String())
	}
	var resp createUserResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.AccessKey == ak {
		t.Error("rotated access key should be different from original")
	}
	if resp.Owner != "charlie" {
		t.Errorf("owner = %q; want charlie", resp.Owner)
	}
}

func TestRotateKeyNotFound(t *testing.T) {
	h := setupUserHandler(t)

	chiCtx := chi.NewRouteContext()
	chiCtx.URLParams.Add("accessKey", "nonexistent")
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/admin/v1/users/nonexistent/keys/rotate", nil)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, chiCtx))
	rr := httptest.NewRecorder()
	h.rotateKey(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("status = %d; want 404", rr.Code)
	}
}
