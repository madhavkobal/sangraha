package integration

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

// buildPresignedURL builds a presigned GET URL using the same SigV4 algorithm as auth/presign.go.
func buildPresignedURL(bucket, key string, expires time.Duration) string {
	now := time.Now().UTC()
	date := now.Format("20060102")
	dateTime := now.Format("20060102T150405Z")
	expireSec := fmt.Sprintf("%d", int(expires.Seconds()))
	region := "us-east-1"

	host := strings.TrimPrefix(s3Endpoint, "http://")
	host = strings.TrimPrefix(host, "https://")

	credScope := date + "/" + region + "/s3/aws4_request"
	credential := rootAK + "/" + credScope

	signedHeaders := "host"

	qs := fmt.Sprintf(
		"X-Amz-Algorithm=AWS4-HMAC-SHA256&X-Amz-Credential=%s&X-Amz-Date=%s&X-Amz-Expires=%s&X-Amz-SignedHeaders=%s",
		credential,
		dateTime,
		expireSec,
		signedHeaders,
	)

	path := "/" + bucket + "/" + key
	canonicalRequest := "GET\n" + path + "\n" + qs + "\nhost:" + host + "\n\nhost\nUNSIGNED-PAYLOAD"

	hh := sha256.Sum256([]byte(canonicalRequest))
	h := hex.EncodeToString(hh[:])
	sts := "AWS4-HMAC-SHA256\n" + dateTime + "\n" + credScope + "\n" + h

	kDate := sigHMAC([]byte("AWS4"+rootSK), []byte(date))
	kRegion := sigHMAC(kDate, []byte(region))
	kService := sigHMAC(kRegion, []byte("s3"))
	kSigning := sigHMAC(kService, []byte("aws4_request"))
	sig := hex.EncodeToString(sigHMAC(kSigning, []byte(sts)))

	return s3Endpoint + path + "?" + qs + "&X-Amz-Signature=" + sig
}

// TestPresignedGetReturnsObjectWithoutAuth verifies that a presigned GET URL
// returns the correct object body without requiring an Authorization header.
func TestPresignedGetReturnsObjectWithoutAuth(t *testing.T) {
	if rootAK == "" || rootSK == "" {
		t.Skip("SANGRAHA_ACCESS_KEY / SANGRAHA_SECRET_KEY not set")
	}

	bucket := fmt.Sprintf("it-presign-%d", time.Now().UnixNano())
	key := "presigned-object.txt"
	content := []byte("hello from presigned URL")

	// Create bucket.
	resp, err := doSignedRequest(http.MethodPut, s3URL(bucket, ""), nil, nil)
	if err != nil {
		t.Fatalf("create bucket: %v", err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("create bucket: want 200 got %d", resp.StatusCode)
	}
	defer func() {
		r, _ := doSignedRequest(http.MethodDelete, s3URL(bucket, key), nil, nil)
		if r != nil {
			_ = r.Body.Close()
		}
		r, _ = doSignedRequest(http.MethodDelete, s3URL(bucket, ""), nil, nil)
		if r != nil {
			_ = r.Body.Close()
		}
	}()

	// Put object.
	resp, err = doSignedRequest(http.MethodPut, s3URL(bucket, key), content, nil)
	if err != nil {
		t.Fatalf("put object: %v", err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("put object: want 200 got %d", resp.StatusCode)
	}

	// Build presigned GET URL.
	presignedURL := buildPresignedURL(bucket, key, 5*time.Minute)

	// Fetch without Authorization header.
	getResp, err := http.Get(presignedURL) //nolint:gosec,noctx
	if err != nil {
		t.Fatalf("presigned get: %v", err)
	}
	defer getResp.Body.Close()

	if getResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(getResp.Body)
		t.Fatalf("presigned get: want 200 got %d: %s", getResp.StatusCode, body)
	}

	got, err := io.ReadAll(getResp.Body)
	if err != nil {
		t.Fatalf("read presigned body: %v", err)
	}
	if string(got) != string(content) {
		t.Errorf("presigned body = %q; want %q", got, content)
	}
}

// TestPresignedURLExpiry verifies that an already-expired presigned URL returns 403.
func TestPresignedURLExpiry(t *testing.T) {
	if rootAK == "" || rootSK == "" {
		t.Skip("SANGRAHA_ACCESS_KEY / SANGRAHA_SECRET_KEY not set")
	}

	bucket := fmt.Sprintf("it-presign-exp-%d", time.Now().UnixNano())
	key := "exp-object.txt"

	// Create bucket and object.
	resp, err := doSignedRequest(http.MethodPut, s3URL(bucket, ""), nil, nil)
	if err != nil {
		t.Fatalf("create bucket: %v", err)
	}
	_ = resp.Body.Close()
	defer func() {
		r, _ := doSignedRequest(http.MethodDelete, s3URL(bucket, key), nil, nil)
		if r != nil {
			_ = r.Body.Close()
		}
		r, _ = doSignedRequest(http.MethodDelete, s3URL(bucket, ""), nil, nil)
		if r != nil {
			_ = r.Body.Close()
		}
	}()

	resp, err = doSignedRequest(http.MethodPut, s3URL(bucket, key), []byte("data"), nil)
	if err != nil {
		t.Fatalf("put: %v", err)
	}
	_ = resp.Body.Close()

	// Generate an already-expired presigned URL by backdating the timestamp.
	expiredURL := buildExpiredPresignedURL(bucket, key)
	getResp, err := http.Get(expiredURL) //nolint:gosec,noctx
	if err != nil {
		t.Fatalf("expired presigned get: %v", err)
	}
	defer getResp.Body.Close()
	_, _ = io.ReadAll(getResp.Body)

	if getResp.StatusCode != http.StatusForbidden && getResp.StatusCode != http.StatusUnauthorized {
		t.Errorf("expired presigned URL: want 403/401 got %d", getResp.StatusCode)
	}
}

// buildExpiredPresignedURL creates a presigned URL that expired 1 hour ago.
func buildExpiredPresignedURL(bucket, key string) string {
	// Backdate by 2 hours so even a 1s expire is already expired.
	now := time.Now().UTC().Add(-2 * time.Hour)
	date := now.Format("20060102")
	dateTime := now.Format("20060102T150405Z")
	region := "us-east-1"
	host := strings.TrimPrefix(s3Endpoint, "http://")
	host = strings.TrimPrefix(host, "https://")

	credScope := date + "/" + region + "/s3/aws4_request"
	credential := rootAK + "/" + credScope

	qs := fmt.Sprintf(
		"X-Amz-Algorithm=AWS4-HMAC-SHA256&X-Amz-Credential=%s&X-Amz-Date=%s&X-Amz-Expires=1&X-Amz-SignedHeaders=host",
		credential, dateTime,
	)
	path := "/" + bucket + "/" + key
	canonicalRequest := "GET\n" + path + "\n" + qs + "\nhost:" + host + "\n\nhost\nUNSIGNED-PAYLOAD"
	hh := sha256.Sum256([]byte(canonicalRequest))
	h := hex.EncodeToString(hh[:])
	sts := "AWS4-HMAC-SHA256\n" + dateTime + "\n" + credScope + "\n" + h
	kDate := sigHMAC([]byte("AWS4"+rootSK), []byte(date))
	kRegion := sigHMAC(kDate, []byte(region))
	kService := sigHMAC(kRegion, []byte("s3"))
	kSigning := sigHMAC(kService, []byte("aws4_request"))
	sig := hex.EncodeToString(sigHMAC(kSigning, []byte(sts)))
	return s3Endpoint + path + "?" + qs + "&X-Amz-Signature=" + sig
}
