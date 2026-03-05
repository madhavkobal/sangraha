package middleware

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/madhavkobal/sangraha/internal/auth"
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

func TestAuthMiddleware_MissingHeader(t *testing.T) {
	ks := setupKeyStore(t)
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := Auth(ks)(next)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("status = %d; want 403", rr.Code)
	}
}

func TestAuthMiddleware_InvalidKey(t *testing.T) {
	ks := setupKeyStore(t)
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := Auth(ks)(next)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "AWS4-HMAC-SHA256 Credential=NOEXIST/20260305/us-east-1/s3/aws4_request,SignedHeaders=host,Signature=abc")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("status = %d; want 403", rr.Code)
	}
}

func TestAuthMiddleware_ValidKey(t *testing.T) {
	ks := setupKeyStore(t)
	ctx := httptest.NewRequest(http.MethodGet, "/", nil).Context()
	ak, _, err := ks.CreateKey(ctx, "testuser", false)
	if err != nil {
		t.Fatalf("CreateKey: %v", err)
	}

	var gotIdentity auth.VerifiedIdentity
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id, ok := IdentityFromContext(r.Context())
		if !ok {
			t.Error("identity not in context")
		}
		gotIdentity = id
		w.WriteHeader(http.StatusOK)
	})
	handler := Auth(ks)(next)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "AWS4-HMAC-SHA256 Credential="+ak+"/20260305/us-east-1/s3/aws4_request,SignedHeaders=host,Signature=abc")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("status = %d; want 200", rr.Code)
	}
	if gotIdentity.AccessKey != ak {
		t.Errorf("identity.AccessKey = %q; want %q", gotIdentity.AccessKey, ak)
	}
}

func TestIdentityFromContextMissing(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	_, ok := IdentityFromContext(req.Context())
	if ok {
		t.Error("IdentityFromContext should return false when no identity is set")
	}
}
