package s3

import (
	"context"
	"encoding/xml"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/madhavkobal/sangraha/internal/audit"
	"github.com/madhavkobal/sangraha/internal/auth"
	"github.com/madhavkobal/sangraha/internal/backend/localfs"
	"github.com/madhavkobal/sangraha/internal/metadata"
	metabbolt "github.com/madhavkobal/sangraha/internal/metadata/bbolt"
	"github.com/madhavkobal/sangraha/internal/storage"
	"github.com/madhavkobal/sangraha/pkg/s3types"
)

// testServer creates a full S3 handler backed by in-process stores.
// It returns the handler and an access key pre-registered in the key store.
// The key is inserted with an empty SigningKey so the auth middleware skips
// full SigV4 verification, allowing tests to use a minimal fake auth header.
func testServer(t *testing.T) (http.Handler, string) {
	t.Helper()
	dir := t.TempDir()

	be, err := localfs.New(filepath.Join(dir, "data"))
	if err != nil {
		t.Fatalf("localfs.New: %v", err)
	}
	meta, err := metabbolt.Open(filepath.Join(dir, "meta.db"))
	if err != nil {
		t.Fatalf("bbolt.Open: %v", err)
	}
	t.Cleanup(func() { _ = meta.Close() })

	engine := storage.New(be, meta, "root")
	ks := auth.NewKeyStore(meta)

	// Insert a test key with no SigningKey — the middleware skips full SigV4
	// verification when SigningKey is empty, so fake auth headers work in tests.
	ak := "TESTKEY1234567890AB"
	rec := metadata.AccessKeyRecord{
		AccessKey:  ak,
		SecretHash: "$2a$12$invalid-hash-for-test-only",
		SigningKey: "", // empty → skip SigV4 verification in middleware
		Owner:      "root",
		CreatedAt:  time.Now().UTC(),
		IsRoot:     true,
	}
	if putErr := meta.PutAccessKey(context.Background(), rec); putErr != nil {
		t.Fatalf("PutAccessKey: %v", putErr)
	}

	// Audit logger that discards output (empty path = discard).
	auditor, err := audit.New("")
	if err != nil {
		t.Fatalf("audit.New: %v", err)
	}

	handler := New(engine, ks, auditor, 1000)
	return handler, ak
}

// authHeader builds a minimal AWS4-HMAC-SHA256 Authorization header for the given access key.
// Works because testServer inserts the key with empty SigningKey (no signature verification).
func authHeader(ak string) string {
	date := time.Now().UTC().Format("20060102")
	return "AWS4-HMAC-SHA256 Credential=" + ak + "/" + date + "/us-east-1/s3/aws4_request, SignedHeaders=host, Signature=fakesig"
}

func TestListBuckets(t *testing.T) {
	h, ak := testServer(t)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", authHeader(ak))
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d; want 200; body: %s", rr.Code, rr.Body.String())
	}
	var result s3types.ListAllMyBucketsResult
	if err := xml.NewDecoder(rr.Body).Decode(&result); err != nil {
		t.Fatalf("decode: %v", err)
	}
}

func TestCreateAndHeadBucket(t *testing.T) {
	h, ak := testServer(t)

	// Create
	req := httptest.NewRequest(http.MethodPut, "/test-bucket", nil)
	req.Header.Set("Authorization", authHeader(ak))
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("CreateBucket status = %d; want 200; body: %s", rr.Code, rr.Body.String())
	}

	// Head
	req2 := httptest.NewRequest(http.MethodHead, "/test-bucket", nil)
	req2.Header.Set("Authorization", authHeader(ak))
	rr2 := httptest.NewRecorder()
	h.ServeHTTP(rr2, req2)

	if rr2.Code != http.StatusOK {
		t.Fatalf("HeadBucket status = %d; want 200", rr2.Code)
	}
}

func TestCreateBucketInvalidName(t *testing.T) {
	h, ak := testServer(t)

	req := httptest.NewRequest(http.MethodPut, "/AB", nil)
	req.Header.Set("Authorization", authHeader(ak))
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("status = %d; want 400", rr.Code)
	}
}

func TestDeleteBucket(t *testing.T) {
	h, ak := testServer(t)

	// Create bucket
	req := httptest.NewRequest(http.MethodPut, "/del-bucket", nil)
	req.Header.Set("Authorization", authHeader(ak))
	h.ServeHTTP(httptest.NewRecorder(), req)

	// Delete bucket
	req2 := httptest.NewRequest(http.MethodDelete, "/del-bucket", nil)
	req2.Header.Set("Authorization", authHeader(ak))
	rr2 := httptest.NewRecorder()
	h.ServeHTTP(rr2, req2)

	if rr2.Code != http.StatusNoContent {
		t.Errorf("DeleteBucket status = %d; want 204", rr2.Code)
	}
}

func TestPutAndGetObject(t *testing.T) {
	h, ak := testServer(t)

	// Create bucket
	req := httptest.NewRequest(http.MethodPut, "/my-bucket", nil)
	req.Header.Set("Authorization", authHeader(ak))
	h.ServeHTTP(httptest.NewRecorder(), req)

	// Put object
	body := "hello world"
	req2 := httptest.NewRequest(http.MethodPut, "/my-bucket/test/key.txt", strings.NewReader(body))
	req2.Header.Set("Authorization", authHeader(ak))
	req2.ContentLength = int64(len(body))
	rr2 := httptest.NewRecorder()
	h.ServeHTTP(rr2, req2)

	if rr2.Code != http.StatusOK {
		t.Fatalf("PutObject status = %d; want 200; body: %s", rr2.Code, rr2.Body.String())
	}
	etag := rr2.Header().Get("ETag")
	if etag == "" {
		t.Error("PutObject should return ETag header")
	}

	// Get object
	req3 := httptest.NewRequest(http.MethodGet, "/my-bucket/test/key.txt", nil)
	req3.Header.Set("Authorization", authHeader(ak))
	rr3 := httptest.NewRecorder()
	h.ServeHTTP(rr3, req3)

	if rr3.Code != http.StatusOK {
		t.Fatalf("GetObject status = %d; want 200", rr3.Code)
	}
	if got := rr3.Body.String(); got != body {
		t.Errorf("body = %q; want %q", got, body)
	}
}

func TestGetObjectNotFound(t *testing.T) {
	h, ak := testServer(t)

	req := httptest.NewRequest(http.MethodPut, "/bucket", nil)
	req.Header.Set("Authorization", authHeader(ak))
	h.ServeHTTP(httptest.NewRecorder(), req)

	req2 := httptest.NewRequest(http.MethodGet, "/bucket/missing", nil)
	req2.Header.Set("Authorization", authHeader(ak))
	rr2 := httptest.NewRecorder()
	h.ServeHTTP(rr2, req2)

	if rr2.Code != http.StatusNotFound {
		t.Errorf("status = %d; want 404", rr2.Code)
	}
}

func TestHeadObject(t *testing.T) {
	h, ak := testServer(t)

	req := httptest.NewRequest(http.MethodPut, "/bucket", nil)
	req.Header.Set("Authorization", authHeader(ak))
	h.ServeHTTP(httptest.NewRecorder(), req)

	body := "data"
	req2 := httptest.NewRequest(http.MethodPut, "/bucket/obj", strings.NewReader(body))
	req2.Header.Set("Authorization", authHeader(ak))
	req2.ContentLength = int64(len(body))
	h.ServeHTTP(httptest.NewRecorder(), req2)

	req3 := httptest.NewRequest(http.MethodHead, "/bucket/obj", nil)
	req3.Header.Set("Authorization", authHeader(ak))
	rr3 := httptest.NewRecorder()
	h.ServeHTTP(rr3, req3)

	if rr3.Code != http.StatusOK {
		t.Errorf("HeadObject status = %d; want 200", rr3.Code)
	}
	if rr3.Header().Get("Content-Length") == "" {
		t.Error("HeadObject should return Content-Length header")
	}
}

func TestDeleteObject(t *testing.T) {
	h, ak := testServer(t)

	req := httptest.NewRequest(http.MethodPut, "/bucket", nil)
	req.Header.Set("Authorization", authHeader(ak))
	h.ServeHTTP(httptest.NewRecorder(), req)

	req2 := httptest.NewRequest(http.MethodPut, "/bucket/obj", strings.NewReader("x"))
	req2.Header.Set("Authorization", authHeader(ak))
	req2.ContentLength = 1
	h.ServeHTTP(httptest.NewRecorder(), req2)

	req3 := httptest.NewRequest(http.MethodDelete, "/bucket/obj", nil)
	req3.Header.Set("Authorization", authHeader(ak))
	rr3 := httptest.NewRecorder()
	h.ServeHTTP(rr3, req3)

	if rr3.Code != http.StatusNoContent {
		t.Errorf("DeleteObject status = %d; want 204", rr3.Code)
	}
}

func TestListObjectsV2(t *testing.T) {
	h, ak := testServer(t)

	req := httptest.NewRequest(http.MethodPut, "/bucket", nil)
	req.Header.Set("Authorization", authHeader(ak))
	h.ServeHTTP(httptest.NewRecorder(), req)

	for _, key := range []string{"a", "b", "c"} {
		r := httptest.NewRequest(http.MethodPut, "/bucket/"+key, strings.NewReader("x"))
		r.Header.Set("Authorization", authHeader(ak))
		r.ContentLength = 1
		h.ServeHTTP(httptest.NewRecorder(), r)
	}

	req2 := httptest.NewRequest(http.MethodGet, "/bucket?list-type=2", nil)
	req2.Header.Set("Authorization", authHeader(ak))
	rr2 := httptest.NewRecorder()
	h.ServeHTTP(rr2, req2)

	if rr2.Code != http.StatusOK {
		t.Fatalf("ListObjectsV2 status = %d; want 200; body: %s", rr2.Code, rr2.Body.String())
	}
	var result s3types.ListBucketResult
	if err := xml.NewDecoder(rr2.Body).Decode(&result); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(result.Contents) != 3 {
		t.Errorf("got %d objects; want 3", len(result.Contents))
	}
}

func TestUnauthenticated(t *testing.T) {
	h, _ := testServer(t)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("unauthenticated status = %d; want 403", rr.Code)
	}
}
func TestCopyObject(t *testing.T) {
	h, ak := testServer(t)

	req := httptest.NewRequest(http.MethodPut, "/bucket", nil)
	req.Header.Set("Authorization", authHeader(ak))
	h.ServeHTTP(httptest.NewRecorder(), req)

	src := "hello copy"
	req2 := httptest.NewRequest(http.MethodPut, "/bucket/src-key", strings.NewReader(src))
	req2.Header.Set("Authorization", authHeader(ak))
	req2.ContentLength = int64(len(src))
	h.ServeHTTP(httptest.NewRecorder(), req2)

	// Copy to new key.
	req3 := httptest.NewRequest(http.MethodPut, "/bucket/dst-key", nil)
	req3.Header.Set("Authorization", authHeader(ak))
	req3.Header.Set("x-amz-copy-source", "/bucket/src-key")
	rr3 := httptest.NewRecorder()
	h.ServeHTTP(rr3, req3)

	if rr3.Code != http.StatusOK {
		t.Fatalf("CopyObject status = %d; want 200; body: %s", rr3.Code, rr3.Body.String())
	}

	// Verify the copy is readable.
	req4 := httptest.NewRequest(http.MethodGet, "/bucket/dst-key", nil)
	req4.Header.Set("Authorization", authHeader(ak))
	rr4 := httptest.NewRecorder()
	h.ServeHTTP(rr4, req4)

	if rr4.Code != http.StatusOK {
		t.Fatalf("GetObject dst-key status = %d; want 200", rr4.Code)
	}
	if rr4.Body.String() != src {
		t.Errorf("body = %q; want %q", rr4.Body.String(), src)
	}
}

func TestDeleteObjects(t *testing.T) {
	h, ak := testServer(t)

	req := httptest.NewRequest(http.MethodPut, "/bucket", nil)
	req.Header.Set("Authorization", authHeader(ak))
	h.ServeHTTP(httptest.NewRecorder(), req)

	for _, key := range []string{"k1", "k2", "k3"} {
		r := httptest.NewRequest(http.MethodPut, "/bucket/"+key, strings.NewReader("x"))
		r.Header.Set("Authorization", authHeader(ak))
		r.ContentLength = 1
		h.ServeHTTP(httptest.NewRecorder(), r)
	}

	// Batch delete k1 and k2.
	delBody := `<Delete><Object><Key>k1</Key></Object><Object><Key>k2</Key></Object></Delete>`
	req2 := httptest.NewRequest(http.MethodPost, "/bucket?delete", strings.NewReader(delBody))
	req2.Header.Set("Authorization", authHeader(ak))
	rr2 := httptest.NewRecorder()
	h.ServeHTTP(rr2, req2)

	if rr2.Code != http.StatusOK {
		t.Fatalf("DeleteObjects status = %d; want 200; body: %s", rr2.Code, rr2.Body.String())
	}
}

func TestListObjectsV1(t *testing.T) {
	h, ak := testServer(t)

	req := httptest.NewRequest(http.MethodPut, "/bucket", nil)
	req.Header.Set("Authorization", authHeader(ak))
	h.ServeHTTP(httptest.NewRecorder(), req)

	for _, key := range []string{"x", "y"} {
		r := httptest.NewRequest(http.MethodPut, "/bucket/"+key, strings.NewReader("v"))
		r.Header.Set("Authorization", authHeader(ak))
		r.ContentLength = 1
		h.ServeHTTP(httptest.NewRecorder(), r)
	}

	req2 := httptest.NewRequest(http.MethodGet, "/bucket", nil)
	req2.Header.Set("Authorization", authHeader(ak))
	rr2 := httptest.NewRecorder()
	h.ServeHTTP(rr2, req2)

	if rr2.Code != http.StatusOK {
		t.Fatalf("ListObjectsV1 status = %d; want 200; body: %s", rr2.Code, rr2.Body.String())
	}
	var result s3types.ListBucketResult
	if err := xml.NewDecoder(rr2.Body).Decode(&result); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(result.Contents) != 2 {
		t.Errorf("got %d objects; want 2", len(result.Contents))
	}
}

func TestHeadBucketNotFound(t *testing.T) {
	h, ak := testServer(t)

	req := httptest.NewRequest(http.MethodHead, "/nonexistent-bucket", nil)
	req.Header.Set("Authorization", authHeader(ak))
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("HeadBucket missing: status = %d; want 404", rr.Code)
	}
}

func TestDeleteBucketNotEmpty(t *testing.T) {
	h, ak := testServer(t)

	req := httptest.NewRequest(http.MethodPut, "/nonempty", nil)
	req.Header.Set("Authorization", authHeader(ak))
	h.ServeHTTP(httptest.NewRecorder(), req)

	req2 := httptest.NewRequest(http.MethodPut, "/nonempty/obj", strings.NewReader("x"))
	req2.Header.Set("Authorization", authHeader(ak))
	req2.ContentLength = 1
	h.ServeHTTP(httptest.NewRecorder(), req2)

	req3 := httptest.NewRequest(http.MethodDelete, "/nonempty", nil)
	req3.Header.Set("Authorization", authHeader(ak))
	rr3 := httptest.NewRecorder()
	h.ServeHTTP(rr3, req3)

	if rr3.Code != http.StatusConflict {
		t.Errorf("DeleteBucket not-empty status = %d; want 409", rr3.Code)
	}
}

func TestMultipartUpload(t *testing.T) {
	h, ak := testServer(t)

	// Create bucket.
	req := httptest.NewRequest(http.MethodPut, "/bucket", nil)
	req.Header.Set("Authorization", authHeader(ak))
	h.ServeHTTP(httptest.NewRecorder(), req)

	// Initiate multipart upload.
	req2 := httptest.NewRequest(http.MethodPost, "/bucket/big-file?uploads", nil)
	req2.Header.Set("Authorization", authHeader(ak))
	rr2 := httptest.NewRecorder()
	h.ServeHTTP(rr2, req2)

	if rr2.Code != http.StatusOK {
		t.Fatalf("CreateMultipartUpload status = %d; want 200; body: %s", rr2.Code, rr2.Body.String())
	}

	// Parse the upload ID.
	var initResult struct {
		UploadID string `xml:"UploadId"`
	}
	if err := xml.NewDecoder(rr2.Body).Decode(&initResult); err != nil {
		t.Fatalf("decode initiate: %v", err)
	}
	uploadID := initResult.UploadID
	if uploadID == "" {
		t.Fatal("UploadId should be set")
	}

	// Upload a part.
	partData := strings.Repeat("Z", 1024)
	req3 := httptest.NewRequest(http.MethodPut, "/bucket/big-file?partNumber=1&uploadId="+uploadID, strings.NewReader(partData))
	req3.Header.Set("Authorization", authHeader(ak))
	req3.ContentLength = int64(len(partData))
	rr3 := httptest.NewRecorder()
	h.ServeHTTP(rr3, req3)

	if rr3.Code != http.StatusOK {
		t.Fatalf("UploadPart status = %d; want 200; body: %s", rr3.Code, rr3.Body.String())
	}
	etag := rr3.Header().Get("ETag")

	// Complete the upload.
	completeBody := `<CompleteMultipartUpload><Part><PartNumber>1</PartNumber><ETag>` + etag + `</ETag></Part></CompleteMultipartUpload>`
	req4 := httptest.NewRequest(http.MethodPost, "/bucket/big-file?uploadId="+uploadID, strings.NewReader(completeBody))
	req4.Header.Set("Authorization", authHeader(ak))
	rr4 := httptest.NewRecorder()
	h.ServeHTTP(rr4, req4)

	if rr4.Code != http.StatusOK {
		t.Fatalf("CompleteMultipartUpload status = %d; want 200; body: %s", rr4.Code, rr4.Body.String())
	}

	// Verify the assembled object is readable.
	req5 := httptest.NewRequest(http.MethodGet, "/bucket/big-file", nil)
	req5.Header.Set("Authorization", authHeader(ak))
	rr5 := httptest.NewRecorder()
	h.ServeHTTP(rr5, req5)

	if rr5.Code != http.StatusOK {
		t.Fatalf("GetObject big-file status = %d; want 200", rr5.Code)
	}
	if rr5.Body.String() != partData {
		t.Errorf("assembled body length mismatch: got %d; want %d", rr5.Body.Len(), len(partData))
	}
}

func TestAbortMultipartUpload(t *testing.T) {
	h, ak := testServer(t)

	req := httptest.NewRequest(http.MethodPut, "/bucket", nil)
	req.Header.Set("Authorization", authHeader(ak))
	h.ServeHTTP(httptest.NewRecorder(), req)

	// Initiate.
	req2 := httptest.NewRequest(http.MethodPost, "/bucket/file?uploads", nil)
	req2.Header.Set("Authorization", authHeader(ak))
	rr2 := httptest.NewRecorder()
	h.ServeHTTP(rr2, req2)

	var initResult struct {
		UploadID string `xml:"UploadId"`
	}
	_ = xml.NewDecoder(rr2.Body).Decode(&initResult)
	uploadID := initResult.UploadID

	// Abort.
	req3 := httptest.NewRequest(http.MethodDelete, "/bucket/file?uploadId="+uploadID, nil)
	req3.Header.Set("Authorization", authHeader(ak))
	rr3 := httptest.NewRecorder()
	h.ServeHTTP(rr3, req3)

	if rr3.Code != http.StatusNoContent {
		t.Errorf("AbortMultipartUpload status = %d; want 204; body: %s", rr3.Code, rr3.Body.String())
	}
}

func TestCreateBucketAlreadyExists(t *testing.T) {
	h, ak := testServer(t)

	req := httptest.NewRequest(http.MethodPut, "/dup-bucket", nil)
	req.Header.Set("Authorization", authHeader(ak))
	h.ServeHTTP(httptest.NewRecorder(), req)

	// Create same bucket again.
	req2 := httptest.NewRequest(http.MethodPut, "/dup-bucket", nil)
	req2.Header.Set("Authorization", authHeader(ak))
	rr2 := httptest.NewRecorder()
	h.ServeHTTP(rr2, req2)

	if rr2.Code != http.StatusConflict {
		t.Errorf("duplicate bucket status = %d; want 409", rr2.Code)
	}
}

func TestHeadObjectNotFound(t *testing.T) {
	h, ak := testServer(t)

	req := httptest.NewRequest(http.MethodPut, "/bucket", nil)
	req.Header.Set("Authorization", authHeader(ak))
	h.ServeHTTP(httptest.NewRecorder(), req)

	req2 := httptest.NewRequest(http.MethodHead, "/bucket/missing-key", nil)
	req2.Header.Set("Authorization", authHeader(ak))
	rr2 := httptest.NewRecorder()
	h.ServeHTTP(rr2, req2)

	if rr2.Code != http.StatusNotFound {
		t.Errorf("HeadObject missing status = %d; want 404", rr2.Code)
	}
}

func TestDeleteObjectMissingBucket(t *testing.T) {
	h, ak := testServer(t)

	req := httptest.NewRequest(http.MethodDelete, "/no-such-bucket/key", nil)
	req.Header.Set("Authorization", authHeader(ak))
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	// DeleteObject on non-existent bucket — storage engine returns nil (idempotent).
	// The bucket doesn't fail at delete-object level (just metadata not found).
	// Either 204 or 404 is acceptable; ensure no 5xx.
	if rr.Code >= 500 {
		t.Errorf("DeleteObject missing bucket: unexpected status %d", rr.Code)
	}
}

func TestCopyObjectMissingSource(t *testing.T) {
	h, ak := testServer(t)

	req := httptest.NewRequest(http.MethodPut, "/bucket", nil)
	req.Header.Set("Authorization", authHeader(ak))
	h.ServeHTTP(httptest.NewRecorder(), req)

	req2 := httptest.NewRequest(http.MethodPut, "/bucket/dst", nil)
	req2.Header.Set("Authorization", authHeader(ak))
	req2.Header.Set("x-amz-copy-source", "/bucket/nonexistent-src")
	rr2 := httptest.NewRecorder()
	h.ServeHTTP(rr2, req2)

	if rr2.Code != http.StatusNotFound {
		t.Errorf("CopyObject missing src: status = %d; want 404", rr2.Code)
	}
}

func TestCopyObjectInvalidSource(t *testing.T) {
	h, ak := testServer(t)

	req := httptest.NewRequest(http.MethodPut, "/bucket", nil)
	req.Header.Set("Authorization", authHeader(ak))
	h.ServeHTTP(httptest.NewRecorder(), req)

	// x-amz-copy-source without a slash (no bucket/key separator).
	req2 := httptest.NewRequest(http.MethodPut, "/bucket/dst", nil)
	req2.Header.Set("Authorization", authHeader(ak))
	req2.Header.Set("x-amz-copy-source", "nodestination")
	rr2 := httptest.NewRecorder()
	h.ServeHTTP(rr2, req2)

	if rr2.Code != http.StatusBadRequest {
		t.Errorf("CopyObject invalid source: status = %d; want 400", rr2.Code)
	}
}

func TestListObjectsV2InvalidMaxKeys(t *testing.T) {
	h, ak := testServer(t)

	req := httptest.NewRequest(http.MethodPut, "/bucket", nil)
	req.Header.Set("Authorization", authHeader(ak))
	h.ServeHTTP(httptest.NewRecorder(), req)

	req2 := httptest.NewRequest(http.MethodGet, "/bucket?list-type=2&max-keys=bad", nil)
	req2.Header.Set("Authorization", authHeader(ak))
	rr2 := httptest.NewRecorder()
	h.ServeHTTP(rr2, req2)

	if rr2.Code != http.StatusBadRequest {
		t.Errorf("ListObjectsV2 invalid max-keys: status = %d; want 400", rr2.Code)
	}
}

func TestListObjectsV2MissingBucket(t *testing.T) {
	h, ak := testServer(t)

	req := httptest.NewRequest(http.MethodGet, "/no-such-bucket?list-type=2", nil)
	req.Header.Set("Authorization", authHeader(ak))
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("ListObjectsV2 missing bucket: status = %d; want 404", rr.Code)
	}
}

func TestListObjectsV1MissingBucket(t *testing.T) {
	h, ak := testServer(t)

	req := httptest.NewRequest(http.MethodGet, "/no-such-bucket", nil)
	req.Header.Set("Authorization", authHeader(ak))
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("ListObjectsV1 missing bucket: status = %d; want 404", rr.Code)
	}
}

func TestListObjectsV2WithPrefix(t *testing.T) {
	h, ak := testServer(t)

	req := httptest.NewRequest(http.MethodPut, "/bucket", nil)
	req.Header.Set("Authorization", authHeader(ak))
	h.ServeHTTP(httptest.NewRecorder(), req)

	for _, key := range []string{"a/1", "a/2", "b/1"} {
		r := httptest.NewRequest(http.MethodPut, "/bucket/"+key, strings.NewReader("x"))
		r.Header.Set("Authorization", authHeader(ak))
		r.ContentLength = 1
		h.ServeHTTP(httptest.NewRecorder(), r)
	}

	req2 := httptest.NewRequest(http.MethodGet, "/bucket?list-type=2&prefix=a/&delimiter=/", nil)
	req2.Header.Set("Authorization", authHeader(ak))
	rr2 := httptest.NewRecorder()
	h.ServeHTTP(rr2, req2)

	if rr2.Code != http.StatusOK {
		t.Fatalf("ListObjectsV2 prefix status = %d; body: %s", rr2.Code, rr2.Body.String())
	}
	var result s3types.ListBucketResult
	if err := xml.NewDecoder(rr2.Body).Decode(&result); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(result.Contents) != 2 {
		t.Errorf("got %d contents; want 2", len(result.Contents))
	}
}

func TestAbortMultipartUploadNotFound(t *testing.T) {
	h, ak := testServer(t)

	req := httptest.NewRequest(http.MethodPut, "/bucket", nil)
	req.Header.Set("Authorization", authHeader(ak))
	h.ServeHTTP(httptest.NewRecorder(), req)

	req2 := httptest.NewRequest(http.MethodDelete, "/bucket/file?uploadId=nonexistent-id", nil)
	req2.Header.Set("Authorization", authHeader(ak))
	rr2 := httptest.NewRecorder()
	h.ServeHTTP(rr2, req2)

	// Aborting a non-existent upload should return 404.
	if rr2.Code != http.StatusNotFound {
		t.Errorf("AbortMultipartUpload not-found: status = %d; want 404", rr2.Code)
	}
}

func TestDeleteBucketNotFound(t *testing.T) {
	h, ak := testServer(t)

	req := httptest.NewRequest(http.MethodDelete, "/no-such-bucket", nil)
	req.Header.Set("Authorization", authHeader(ak))
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("DeleteBucket not-found: status = %d; want 404", rr.Code)
	}
}

func TestUploadPartInvalidNumber(t *testing.T) {
	h, ak := testServer(t)

	req := httptest.NewRequest(http.MethodPut, "/bucket", nil)
	req.Header.Set("Authorization", authHeader(ak))
	h.ServeHTTP(httptest.NewRecorder(), req)

	req2 := httptest.NewRequest(http.MethodPut, "/bucket/file?partNumber=bad&uploadId=fake", nil)
	req2.Header.Set("Authorization", authHeader(ak))
	rr2 := httptest.NewRecorder()
	h.ServeHTTP(rr2, req2)

	if rr2.Code != http.StatusBadRequest {
		t.Errorf("UploadPart bad partNumber: status = %d; want 400", rr2.Code)
	}
}

func TestGetObjectWithRange(t *testing.T) {
	h, ak := testServer(t)

	req := httptest.NewRequest(http.MethodPut, "/bucket", nil)
	req.Header.Set("Authorization", authHeader(ak))
	h.ServeHTTP(httptest.NewRecorder(), req)

	body := "hello world!"
	req2 := httptest.NewRequest(http.MethodPut, "/bucket/obj", strings.NewReader(body))
	req2.Header.Set("Authorization", authHeader(ak))
	req2.ContentLength = int64(len(body))
	h.ServeHTTP(httptest.NewRecorder(), req2)

	req3 := httptest.NewRequest(http.MethodGet, "/bucket/obj", nil)
	req3.Header.Set("Authorization", authHeader(ak))
	req3.Header.Set("Range", "bytes=0-4")
	rr3 := httptest.NewRecorder()
	h.ServeHTTP(rr3, req3)

	// 206 Partial Content or 200 OK both acceptable.
	if rr3.Code >= 500 {
		t.Errorf("GetObject with Range: unexpected status %d", rr3.Code)
	}
}

func TestPostBucketInvalidOperation(t *testing.T) {
	h, ak := testServer(t)

	req := httptest.NewRequest(http.MethodPut, "/bucket", nil)
	req.Header.Set("Authorization", authHeader(ak))
	h.ServeHTTP(httptest.NewRecorder(), req)

	// POST to bucket without ?delete should return 400.
	req2 := httptest.NewRequest(http.MethodPost, "/bucket", nil)
	req2.Header.Set("Authorization", authHeader(ak))
	rr2 := httptest.NewRecorder()
	h.ServeHTTP(rr2, req2)

	if rr2.Code != http.StatusBadRequest {
		t.Errorf("invalid bucket POST: status = %d; want 400", rr2.Code)
	}
}

func TestCreateMultipartUploadMissingBucket(t *testing.T) {
	h, ak := testServer(t)

	req := httptest.NewRequest(http.MethodPost, "/no-such-bucket/file?uploads", nil)
	req.Header.Set("Authorization", authHeader(ak))
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("CreateMultipartUpload missing bucket: status = %d; want 404", rr.Code)
	}
}

func TestUploadPartMissingUploadID(t *testing.T) {
	h, ak := testServer(t)

	req := httptest.NewRequest(http.MethodPut, "/bucket", nil)
	req.Header.Set("Authorization", authHeader(ak))
	h.ServeHTTP(httptest.NewRecorder(), req)

	// Include ?uploadId= (empty) so the router dispatches to uploadPart handler,
	// which validates that uploadId must not be empty.
	req2 := httptest.NewRequest(http.MethodPut, "/bucket/file?partNumber=1&uploadId=", strings.NewReader("data"))
	req2.Header.Set("Authorization", authHeader(ak))
	rr2 := httptest.NewRecorder()
	h.ServeHTTP(rr2, req2)

	if rr2.Code != http.StatusBadRequest {
		t.Errorf("UploadPart empty uploadId: status = %d; want 400", rr2.Code)
	}
}

func TestCompleteMultipartUploadEmptyID(t *testing.T) {
	h, ak := testServer(t)

	req := httptest.NewRequest(http.MethodPut, "/bucket", nil)
	req.Header.Set("Authorization", authHeader(ak))
	h.ServeHTTP(httptest.NewRecorder(), req)

	// POST with empty uploadId= should return 400.
	req2 := httptest.NewRequest(http.MethodPost, "/bucket/file?uploadId=", nil)
	req2.Header.Set("Authorization", authHeader(ak))
	rr2 := httptest.NewRecorder()
	h.ServeHTTP(rr2, req2)

	if rr2.Code != http.StatusBadRequest {
		t.Errorf("CompleteMultipartUpload empty uploadId: status = %d; want 400", rr2.Code)
	}
}

func TestCompleteMultipartUploadBadXML(t *testing.T) {
	h, ak := testServer(t)

	req := httptest.NewRequest(http.MethodPut, "/bucket", nil)
	req.Header.Set("Authorization", authHeader(ak))
	h.ServeHTTP(httptest.NewRecorder(), req)

	req2 := httptest.NewRequest(http.MethodPost, "/bucket/big-file?uploads", nil)
	req2.Header.Set("Authorization", authHeader(ak))
	rr2 := httptest.NewRecorder()
	h.ServeHTTP(rr2, req2)
	var initResult struct {
		UploadID string `xml:"UploadId"`
	}
	_ = xml.NewDecoder(rr2.Body).Decode(&initResult)
	if initResult.UploadID == "" {
		t.Fatal("expected UploadID in init response")
	}

	req3 := httptest.NewRequest(http.MethodPost, "/bucket/big-file?uploadId="+initResult.UploadID,
		strings.NewReader("<<<not-valid-xml>>>"))
	req3.Header.Set("Authorization", authHeader(ak))
	rr3 := httptest.NewRecorder()
	h.ServeHTTP(rr3, req3)

	if rr3.Code != http.StatusBadRequest {
		t.Errorf("CompleteMultipartUpload bad XML: status = %d; want 400", rr3.Code)
	}
}

func TestDeleteObjectsInvalidXML(t *testing.T) {
	h, ak := testServer(t)

	req := httptest.NewRequest(http.MethodPut, "/bucket", nil)
	req.Header.Set("Authorization", authHeader(ak))
	h.ServeHTTP(httptest.NewRecorder(), req)

	req2 := httptest.NewRequest(http.MethodPost, "/bucket?delete", strings.NewReader("not-xml"))
	req2.Header.Set("Authorization", authHeader(ak))
	rr2 := httptest.NewRecorder()
	h.ServeHTTP(rr2, req2)

	if rr2.Code != http.StatusBadRequest {
		t.Errorf("DeleteObjects bad XML: status = %d; want 400", rr2.Code)
	}
}

func TestObjectUserMeta(t *testing.T) {
	h, ak := testServer(t)

	req := httptest.NewRequest(http.MethodPut, "/bucket", nil)
	req.Header.Set("Authorization", authHeader(ak))
	h.ServeHTTP(httptest.NewRecorder(), req)

	req2 := httptest.NewRequest(http.MethodPut, "/bucket/obj", strings.NewReader("data"))
	req2.Header.Set("Authorization", authHeader(ak))
	req2.Header.Set("x-amz-meta-author", "alice")
	req2.ContentLength = 4
	h.ServeHTTP(httptest.NewRecorder(), req2)

	req3 := httptest.NewRequest(http.MethodGet, "/bucket/obj", nil)
	req3.Header.Set("Authorization", authHeader(ak))
	rr3 := httptest.NewRecorder()
	h.ServeHTTP(rr3, req3)

	if rr3.Code != http.StatusOK {
		t.Fatalf("GetObject status = %d; want 200", rr3.Code)
	}
	if rr3.Header().Get("x-amz-meta-author") != "alice" {
		t.Errorf("x-amz-meta-author = %q; want alice", rr3.Header().Get("x-amz-meta-author"))
	}
}
