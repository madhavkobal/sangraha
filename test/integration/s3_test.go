// Package integration runs integration tests against a live sangraha binary.
// Tests use raw net/http with full AWS SigV4 request signing.
//
// The server is started and torn down by integration-test.sh.
// The S3_ENDPOINT and ADMIN_ENDPOINT env vars can override the default ports.
package integration

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"
)

var (
	s3Endpoint    = "http://localhost:19000"
	adminEndpoint = "http://localhost:19001"

	// rootAK / rootSK are provisioned by integration-test.sh via env vars.
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

// doSignedRequest makes an HTTP request with proper AWS SigV4 signing.
// bodyBytes may be nil for requests with no body (GET, DELETE, HEAD).
func doSignedRequest(method, rawURL string, bodyBytes []byte, extraHeaders map[string]string) (*http.Response, error) {
	var body io.Reader
	if len(bodyBytes) > 0 {
		body = bytes.NewReader(bodyBytes)
	}
	req, err := http.NewRequestWithContext(context.Background(), method, rawURL, body)
	if err != nil {
		return nil, err
	}
	if len(bodyBytes) > 0 {
		req.ContentLength = int64(len(bodyBytes))
	}
	for k, v := range extraHeaders {
		req.Header.Set(k, v)
	}
	sigV4Sign(req, bodyBytes)
	return http.DefaultClient.Do(req)
}

// sigV4Sign adds AWS Signature Version 4 authorization headers to req.
// It signs: host, x-amz-content-sha256, x-amz-date.
func sigV4Sign(req *http.Request, bodyBytes []byte) {
	now := time.Now().UTC()
	dateTime := now.Format("20060102T150405Z")
	date := now.Format("20060102")

	// Payload hash.
	h := sha256.New()
	h.Write(bodyBytes)
	payloadHash := hex.EncodeToString(h.Sum(nil))

	req.Header.Set("X-Amz-Date", dateTime)
	req.Header.Set("X-Amz-Content-Sha256", payloadHash)

	host := req.URL.Host
	if req.Host != "" {
		host = req.Host
	}

	// Canonical headers (alphabetically: host, x-amz-content-sha256, x-amz-date).
	canonHeaders := "host:" + host + "\n" +
		"x-amz-content-sha256:" + payloadHash + "\n" +
		"x-amz-date:" + dateTime + "\n"
	signedHeaders := "host;x-amz-content-sha256;x-amz-date"

	// Canonical query string.
	q := req.URL.Query()
	pairs := make([]string, 0, len(q))
	for k, vals := range q {
		for _, v := range vals {
			pairs = append(pairs, s3Encode(k)+"="+s3Encode(v))
		}
	}
	sort.Strings(pairs)
	canonQuery := strings.Join(pairs, "&")

	// Canonical URI.
	uri := req.URL.Path
	if uri == "" {
		uri = "/"
	}

	// Canonical request.
	canonReq := strings.Join([]string{req.Method, uri, canonQuery, canonHeaders, signedHeaders, payloadHash}, "\n")

	// String to sign.
	credScope := date + "/us-east-1/s3/aws4_request"
	h2 := sha256.New()
	h2.Write([]byte(canonReq))
	sts := "AWS4-HMAC-SHA256\n" + dateTime + "\n" + credScope + "\n" + hex.EncodeToString(h2.Sum(nil))

	// Derive signing key.
	kDate := sigHMAC([]byte("AWS4"+rootSK), []byte(date))
	kRegion := sigHMAC(kDate, []byte("us-east-1"))
	kService := sigHMAC(kRegion, []byte("s3"))
	kSigning := sigHMAC(kService, []byte("aws4_request"))
	sig := hex.EncodeToString(sigHMAC(kSigning, []byte(sts)))

	req.Header.Set("Authorization", fmt.Sprintf(
		"AWS4-HMAC-SHA256 Credential=%s/%s, SignedHeaders=%s, Signature=%s",
		rootAK, credScope, signedHeaders, sig,
	))
}

// s3Encode percent-encodes a string per SigV4 URI encoding rules.
// Unreserved characters (A-Z, a-z, 0-9, -, _, ., ~) are left as-is.
func s3Encode(s string) string {
	var sb strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		if (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') ||
			c == '-' || c == '_' || c == '.' || c == '~' {
			sb.WriteByte(c)
		} else {
			fmt.Fprintf(&sb, "%%%02X", c)
		}
	}
	return sb.String()
}

func sigHMAC(key, data []byte) []byte {
	h := hmac.New(sha256.New, key)
	h.Write(data)
	return h.Sum(nil)
}

// --- Helpers ---

func createBucket(t *testing.T, bucket string) {
	t.Helper()
	resp, err := doSignedRequest(http.MethodPut, s3URL(bucket, ""), nil, nil)
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
	resp, err := doSignedRequest(http.MethodPut, s3URL(bucket, key), []byte(body), nil)
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
