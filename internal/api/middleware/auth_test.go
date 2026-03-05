package middleware

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/madhavkobal/sangraha/internal/auth"
	bboltstore "github.com/madhavkobal/sangraha/internal/metadata/bbolt"
)

// buildSigV4Header computes a real AWS4-HMAC-SHA256 Authorization header.
func buildSigV4Header(r *http.Request, accessKey, secretKey string, now time.Time) string {
	dateStr := now.UTC().Format("20060102")
	amzDate := now.UTC().Format("20060102T150405Z")
	region := "us-east-1"
	service := "s3"

	r.Header.Set("X-Amz-Date", amzDate)
	r.Header.Set("X-Amz-Content-Sha256", "UNSIGNED-PAYLOAD")

	signedHeaders := "host;x-amz-content-sha256;x-amz-date"
	canonicalHeaders := "host:" + r.Host + "\nx-amz-content-sha256:UNSIGNED-PAYLOAD\nx-amz-date:" + amzDate + "\n"
	canonicalReq := r.Method + "\n" + r.URL.Path + "\n\n" + canonicalHeaders + "\n" + signedHeaders + "\nUNSIGNED-PAYLOAD"
	credScope := dateStr + "/" + region + "/" + service + "/aws4_request"
	stringToSign := "AWS4-HMAC-SHA256\n" + amzDate + "\n" + credScope + "\n" + hexSHA256(canonicalReq)

	kDate := hmacBytes([]byte("AWS4"+secretKey), []byte(dateStr))
	kRegion := hmacBytes(kDate, []byte(region))
	kService := hmacBytes(kRegion, []byte(service))
	kSigning := hmacBytes(kService, []byte("aws4_request"))
	sig := hex.EncodeToString(hmacBytes(kSigning, []byte(stringToSign)))

	return "AWS4-HMAC-SHA256 Credential=" + accessKey + "/" + credScope + ", SignedHeaders=" + signedHeaders + ", Signature=" + sig
}

func hmacBytes(key, data []byte) []byte {
	h := hmac.New(sha256.New, key)
	h.Write(data)
	return h.Sum(nil)
}

func hexSHA256(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}

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
	ak, sk, err := ks.CreateKey(ctx, "testuser", false)
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
	now := time.Now().UTC()
	req.Header.Set("Authorization", buildSigV4Header(req, ak, sk, now))
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
