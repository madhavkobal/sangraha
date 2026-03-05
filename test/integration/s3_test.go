// Package integration runs integration tests against a live sangraha binary.
// Tests use raw net/http (no SDK) to stay dependency-free.
//
// The server is started and torn down by TestMain.
// The S3_ENDPOINT and ADMIN_ENDPOINT env vars can override the default ports.
package integration

import (
	"bytes"
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

var (
	s3Endpoint    = "http://localhost:19000"
	adminEndpoint = "http://localhost:19001"

	// rootAK / rootSK are provisioned by TestMain via env vars.
	rootAK = ""
	rootSK = ""
)

func init() {
	if v := os.Getenv("S3_ENDPOINT"); v != "" {
		s3Endpoint = v
	}
	if v := os.Getenv("ADMIN_ENDPOINT"); v != "" {
		adminEndpoint = v
	}
	if v := os.Getenv("SANGRAHA_ACCESS_KEY"); v != "" {
		rootAK = v
	}
	if v := os.Getenv("SANGRAHA_SECRET_KEY"); v != "" {
		rootSK = v
	}
}

// TestMain waits for the server and provides a test root. It does NOT start
// the server itself — that is done by integration-test.sh.
func TestMain(m *testing.M) {
	if os.Getenv("SANGRAHA_INTEGRATION") == "" {
		// Skip unless explicitly enabled via the integration test runner.
		fmt.Println("SANGRAHA_INTEGRATION not set; skipping integration tests.")
		os.Exit(0)
	}
	if err := waitForServer(adminEndpoint+"/admin/v1/health", 15*time.Second); err != nil {
		fmt.Fprintf(os.Stderr, "server did not become ready: %v\n", err)
		os.Exit(1)
	}
	os.Exit(m.Run())
}

// waitForServer polls the given health URL until it returns 200 or timeout.
func waitForServer(url string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		resp, err := http.Get(url) //nolint:gosec,noctx
		if err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return nil
			}
		}
		time.Sleep(200 * time.Millisecond)
	}
	return fmt.Errorf("server not ready after %s", timeout)
}

// s3URL builds an S3 endpoint URL for the given bucket and optional key.
func s3URL(bucket, key string) string {
	if key == "" {
		return s3Endpoint + "/" + bucket
	}
	return s3Endpoint + "/" + bucket + "/" + key
}

// authHeaders builds a minimal Authorization header. Uses SigV4 pre-signed
// format without body signing (acceptable for integration tests where the
// server verifies only the access key lookup).
func authHeaders(ak string) map[string]string {
	date := time.Now().UTC().Format("20060102")
	credScope := date + "/us-east-1/s3/aws4_request"
	return map[string]string{
		"Authorization": fmt.Sprintf(
			"AWS4-HMAC-SHA256 Credential=%s/%s, SignedHeaders=host;x-amz-date, Signature=fakesig",
			ak, credScope,
		),
		"X-Amz-Date": time.Now().UTC().Format("20060102T150405Z"),
	}
}

func doRequest(method, url string, body io.Reader, headers map[string]string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(context.Background(), method, url, body)
	if err != nil {
		return nil, err
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	return http.DefaultClient.Do(req)
}

// --- Helpers ---

func createBucket(t *testing.T, bucket string) {
	t.Helper()
	resp, err := doRequest(http.MethodPut, s3URL(bucket, ""), nil, authHeaders(rootAK))
	if err != nil {
		t.Fatalf("createBucket %s: %v", bucket, err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("createBucket %s: status %d", bucket, resp.StatusCode)
	}
}

func putObject(t *testing.T, bucket, key, body string) {
	t.Helper()
	h := authHeaders(rootAK)
	h["Content-Length"] = fmt.Sprintf("%d", len(body))
	resp, err := doRequest(http.MethodPut, s3URL(bucket, key), strings.NewReader(body), h)
	if err != nil {
		t.Fatalf("putObject %s/%s: %v", bucket, key, err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("putObject %s/%s: status %d", bucket, key, resp.StatusCode)
	}
}

// --- Tests ---

func TestIntegrationHealthEndpoint(t *testing.T) {
	resp, err := http.Get(adminEndpoint + "/admin/v1/health") //nolint:gosec,noctx
	if err != nil {
		t.Fatalf("GET /admin/v1/health: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("health status = %d; want 200", resp.StatusCode)
	}
}

func TestIntegrationReadyEndpoint(t *testing.T) {
	resp, err := http.Get(adminEndpoint + "/admin/v1/ready") //nolint:gosec,noctx
	if err != nil {
		t.Fatalf("GET /admin/v1/ready: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("ready status = %d; want 200", resp.StatusCode)
	}
}

func TestIntegrationListBuckets(t *testing.T) {
	resp, err := doRequest(http.MethodGet, s3Endpoint+"/", nil, authHeaders(rootAK))
	if err != nil {
		t.Fatalf("ListBuckets: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("ListBuckets status = %d; want 200", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	if !bytes.Contains(body, []byte("ListAllMyBucketsResult")) {
		t.Errorf("ListBuckets response does not contain expected XML element")
	}
}

func TestIntegrationCreateAndDeleteBucket(t *testing.T) {
	bucket := fmt.Sprintf("integ-bucket-%d", time.Now().UnixNano())
	createBucket(t, bucket)

	// HeadBucket should succeed.
	resp, err := doRequest(http.MethodHead, s3URL(bucket, ""), nil, authHeaders(rootAK))
	if err != nil {
		t.Fatalf("HeadBucket: %v", err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("HeadBucket status = %d; want 200", resp.StatusCode)
	}

	// DeleteBucket.
	resp2, err := doRequest(http.MethodDelete, s3URL(bucket, ""), nil, authHeaders(rootAK))
	if err != nil {
		t.Fatalf("DeleteBucket: %v", err)
	}
	_ = resp2.Body.Close()
	if resp2.StatusCode != http.StatusNoContent {
		t.Errorf("DeleteBucket status = %d; want 204", resp2.StatusCode)
	}
}

func TestIntegrationPutGetDeleteObject(t *testing.T) {
	bucket := fmt.Sprintf("integ-obj-%d", time.Now().UnixNano())
	createBucket(t, bucket)
	t.Cleanup(func() {
		// Best-effort cleanup.
		if r, err := doRequest(http.MethodDelete, s3URL(bucket, "testkey"), nil, authHeaders(rootAK)); err == nil {
			_ = r.Body.Close()
		}
		if r, err := doRequest(http.MethodDelete, s3URL(bucket, ""), nil, authHeaders(rootAK)); err == nil {
			_ = r.Body.Close()
		}
	})

	// PutObject.
	putObject(t, bucket, "testkey", "hello integration")

	// GetObject.
	resp, err := doRequest(http.MethodGet, s3URL(bucket, "testkey"), nil, authHeaders(rootAK))
	if err != nil {
		t.Fatalf("GetObject: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GetObject status = %d; want 200", resp.StatusCode)
	}
	got, _ := io.ReadAll(resp.Body)
	if string(got) != "hello integration" {
		t.Errorf("GetObject body = %q; want %q", string(got), "hello integration")
	}

	// DeleteObject.
	resp2, err := doRequest(http.MethodDelete, s3URL(bucket, "testkey"), nil, authHeaders(rootAK))
	if err != nil {
		t.Fatalf("DeleteObject: %v", err)
	}
	_ = resp2.Body.Close()
	if resp2.StatusCode != http.StatusNoContent {
		t.Errorf("DeleteObject status = %d; want 204", resp2.StatusCode)
	}
}

func TestIntegrationListObjectsV2(t *testing.T) {
	bucket := fmt.Sprintf("integ-list-%d", time.Now().UnixNano())
	createBucket(t, bucket)

	for _, key := range []string{"a/1", "a/2", "b/1"} {
		putObject(t, bucket, key, "x")
	}

	resp, err := doRequest(http.MethodGet, s3URL(bucket, "")+"?list-type=2", nil, authHeaders(rootAK))
	if err != nil {
		t.Fatalf("ListObjectsV2: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("ListObjectsV2 status = %d; want 200", resp.StatusCode)
	}

	var result struct {
		Contents []struct{ Key string } `xml:"Contents"`
	}
	if err := xml.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(result.Contents) != 3 {
		t.Errorf("got %d contents; want 3", len(result.Contents))
	}
}

func TestIntegrationUnauthenticated(t *testing.T) {
	resp, err := http.Get(s3Endpoint + "/") //nolint:gosec,noctx
	if err != nil {
		t.Fatalf("GET /: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("unauthenticated status = %d; want 403", resp.StatusCode)
	}
}

func TestIntegrationHeadObjectNotFound(t *testing.T) {
	bucket := fmt.Sprintf("integ-head-%d", time.Now().UnixNano())
	createBucket(t, bucket)

	resp, err := doRequest(http.MethodHead, s3URL(bucket, "missing"), nil, authHeaders(rootAK))
	if err != nil {
		t.Fatalf("HeadObject: %v", err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("HeadObject missing: status = %d; want 404", resp.StatusCode)
	}
}

// tempDir creates a temporary directory for integration test data.
func tempDir(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("", "sangraha-integ-*")
	if err != nil {
		t.Fatalf("MkdirTemp: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	return dir
}

// configFile writes a minimal YAML config and returns the path.
func configFile(t *testing.T, dataDir, metaPath string, s3Port, adminPort int) string {
	t.Helper()
	content := fmt.Sprintf(`server:
  s3_address: ":%d"
  admin_address: ":%d"
  tls:
    enabled: false
storage:
  backend: localfs
  data_dir: "%s"
metadata:
  path: "%s"
auth:
  root_access_key: "rootkey"
logging:
  level: error
  format: text
`, s3Port, adminPort, filepath.ToSlash(dataDir), filepath.ToSlash(metaPath))

	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return path
}
