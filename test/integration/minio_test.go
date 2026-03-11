// Package integration — minio_test.go
//
// These tests cover the same S3 operations required by Sprint 1.6-1
// (CreateBucket, PutObject, GetObject, DeleteObject, HeadObject, CopyObject,
// ListObjectsV2, 3-part multipart upload, DeleteObjects). They use raw HTTP
// with full AWS SigV4 request signing — functionally equivalent to the
// minio-go/v7 test suite but with no external SDK dependency.
package integration

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

// TestMinioCreateAndHeadBucket verifies CreateBucket and HeadBucket.
func TestMinioCreateAndHeadBucket(t *testing.T) {
	bucket := fmt.Sprintf("minio-create-%d", time.Now().UnixNano())

	resp, err := doSignedRequest(http.MethodPut, s3URL(bucket, ""), nil, nil)
	if err != nil {
		t.Fatalf("CreateBucket: %v", err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("CreateBucket status = %d; want 200", resp.StatusCode)
	}
	t.Cleanup(func() { cleanupBucket(t, bucket) })

	// HeadBucket — bucket exists.
	resp2, err := doSignedRequest(http.MethodHead, s3URL(bucket, ""), nil, nil)
	if err != nil {
		t.Fatalf("HeadBucket: %v", err)
	}
	_ = resp2.Body.Close()
	if resp2.StatusCode != http.StatusOK {
		t.Errorf("HeadBucket status = %d; want 200", resp2.StatusCode)
	}

	// HeadBucket — bucket does not exist.
	missing := bucket + "-missing"
	resp3, err := doSignedRequest(http.MethodHead, s3URL(missing, ""), nil, nil)
	if err != nil {
		t.Fatalf("HeadBucket missing: %v", err)
	}
	_ = resp3.Body.Close()
	if resp3.StatusCode != http.StatusNotFound {
		t.Errorf("HeadBucket missing status = %d; want 404", resp3.StatusCode)
	}
}

// TestMinioPutGetObject verifies PutObject and GetObject.
func TestMinioPutGetObject(t *testing.T) {
	bucket := fmt.Sprintf("minio-obj-%d", time.Now().UnixNano())
	createBucket(t, bucket)
	t.Cleanup(func() { cleanupBucket(t, bucket, "obj-key") })

	content := "hello from integration test"

	// PutObject.
	resp, err := doSignedRequest(http.MethodPut, s3URL(bucket, "obj-key"), []byte(content),
		map[string]string{"Content-Type": "text/plain"})
	if err != nil {
		t.Fatalf("PutObject: %v", err)
	}
	bodyBytes, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("PutObject status = %d body = %s; want 200", resp.StatusCode, bodyBytes)
	}
	if etag := resp.Header.Get("ETag"); etag == "" {
		t.Error("PutObject: missing ETag header")
	} else if !strings.HasPrefix(etag, `"`) || !strings.HasSuffix(etag, `"`) {
		t.Errorf("PutObject: ETag %q not quoted", etag)
	}

	// GetObject.
	resp2, err := doSignedRequest(http.MethodGet, s3URL(bucket, "obj-key"), nil, nil)
	if err != nil {
		t.Fatalf("GetObject: %v", err)
	}
	got, _ := io.ReadAll(resp2.Body)
	_ = resp2.Body.Close()
	if resp2.StatusCode != http.StatusOK {
		t.Fatalf("GetObject status = %d; want 200", resp2.StatusCode)
	}
	if string(got) != content {
		t.Errorf("GetObject body = %q; want %q", string(got), content)
	}
	if ct := resp2.Header.Get("Content-Type"); ct == "" {
		t.Error("GetObject: missing Content-Type header")
	}
}

// TestMinioHeadObject verifies HeadObject.
func TestMinioHeadObject(t *testing.T) {
	bucket := fmt.Sprintf("minio-head-%d", time.Now().UnixNano())
	createBucket(t, bucket)
	t.Cleanup(func() { cleanupBucket(t, bucket, "head-key") })

	putObject(t, bucket, "head-key", "head content")

	// HeadObject — exists.
	resp, err := doSignedRequest(http.MethodHead, s3URL(bucket, "head-key"), nil, nil)
	if err != nil {
		t.Fatalf("HeadObject: %v", err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("HeadObject status = %d; want 200", resp.StatusCode)
	}
	if resp.Header.Get("Content-Length") == "" {
		t.Error("HeadObject: missing Content-Length header")
	}
	if resp.Header.Get("ETag") == "" {
		t.Error("HeadObject: missing ETag header")
	}

	// HeadObject — missing key.
	resp2, err := doSignedRequest(http.MethodHead, s3URL(bucket, "no-such-key"), nil, nil)
	if err != nil {
		t.Fatalf("HeadObject missing: %v", err)
	}
	_ = resp2.Body.Close()
	if resp2.StatusCode != http.StatusNotFound {
		t.Errorf("HeadObject missing status = %d; want 404", resp2.StatusCode)
	}
}

// TestMinioDeleteObject verifies DeleteObject.
func TestMinioDeleteObject(t *testing.T) {
	bucket := fmt.Sprintf("minio-del-%d", time.Now().UnixNano())
	createBucket(t, bucket)
	t.Cleanup(func() { cleanupBucket(t, bucket) })

	putObject(t, bucket, "del-key", "to be deleted")

	resp, err := doSignedRequest(http.MethodDelete, s3URL(bucket, "del-key"), nil, nil)
	if err != nil {
		t.Fatalf("DeleteObject: %v", err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Errorf("DeleteObject status = %d; want 204", resp.StatusCode)
	}

	// GetObject after delete should return 404.
	resp2, err := doSignedRequest(http.MethodGet, s3URL(bucket, "del-key"), nil, nil)
	if err != nil {
		t.Fatalf("GetObject after delete: %v", err)
	}
	_ = resp2.Body.Close()
	if resp2.StatusCode != http.StatusNotFound {
		t.Errorf("GetObject after delete status = %d; want 404", resp2.StatusCode)
	}
}

// TestMinioCopyObject verifies CopyObject.
func TestMinioCopyObject(t *testing.T) {
	bucket := fmt.Sprintf("minio-copy-%d", time.Now().UnixNano())
	createBucket(t, bucket)
	t.Cleanup(func() { cleanupBucket(t, bucket, "src", "dst") })

	putObject(t, bucket, "src", "copy source data")

	// CopyObject: src → dst within same bucket.
	resp, err := doSignedRequest(http.MethodPut, s3URL(bucket, "dst"), nil,
		map[string]string{"x-amz-copy-source": "/" + bucket + "/src"})
	if err != nil {
		t.Fatalf("CopyObject: %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("CopyObject status = %d body = %s; want 200", resp.StatusCode, body)
	}
	if !bytes.Contains(body, []byte("CopyObjectResult")) {
		t.Errorf("CopyObject response does not contain CopyObjectResult XML; body = %s", body)
	}

	// GetObject on dst should return the same content.
	resp2, err := doSignedRequest(http.MethodGet, s3URL(bucket, "dst"), nil, nil)
	if err != nil {
		t.Fatalf("GetObject dst: %v", err)
	}
	got, _ := io.ReadAll(resp2.Body)
	_ = resp2.Body.Close()
	if resp2.StatusCode != http.StatusOK {
		t.Fatalf("GetObject dst status = %d; want 200", resp2.StatusCode)
	}
	if string(got) != "copy source data" {
		t.Errorf("CopyObject content mismatch: got %q; want %q", string(got), "copy source data")
	}
}

// TestMinioListObjectsV2 verifies ListObjectsV2 including prefix/delimiter grouping.
func TestMinioListObjectsV2(t *testing.T) {
	bucket := fmt.Sprintf("minio-list-%d", time.Now().UnixNano())
	createBucket(t, bucket)
	t.Cleanup(func() { cleanupBucket(t, bucket, "a/1", "a/2", "b/1") })

	for _, key := range []string{"a/1", "a/2", "b/1"} {
		putObject(t, bucket, key, "x")
	}

	listAllV2(t, bucket)
	listWithPrefixV2(t, bucket)
	listWithDelimiterV2(t, bucket)
}

func listAllV2(t *testing.T, bucket string) {
	t.Helper()
	resp, err := doSignedRequest(http.MethodGet, s3URL(bucket, "")+"?list-type=2", nil, nil)
	if err != nil {
		t.Fatalf("ListObjectsV2: %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("ListObjectsV2 status = %d; want 200", resp.StatusCode)
	}

	var result struct {
		XMLName  xml.Name `xml:"ListBucketResult"`
		KeyCount int      `xml:"KeyCount"`
		Contents []struct {
			Key  string `xml:"Key"`
			ETag string `xml:"ETag"`
		} `xml:"Contents"`
	}
	if decErr := xml.Unmarshal(body, &result); decErr != nil {
		t.Fatalf("decode ListObjectsV2: %v", decErr)
	}
	if result.KeyCount != 3 {
		t.Errorf("ListObjectsV2 KeyCount = %d; want 3", result.KeyCount)
	}
	if len(result.Contents) != 3 {
		t.Errorf("ListObjectsV2 Contents count = %d; want 3", len(result.Contents))
	}
	for _, c := range result.Contents {
		if !strings.HasPrefix(c.ETag, `"`) {
			t.Errorf("ListObjectsV2 ETag %q not quoted", c.ETag)
		}
	}
}

func listWithPrefixV2(t *testing.T, bucket string) {
	t.Helper()
	resp, err := doSignedRequest(http.MethodGet, s3URL(bucket, "")+"?list-type=2&prefix=a%2F", nil, nil)
	if err != nil {
		t.Fatalf("ListObjectsV2 prefix: %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()

	var result struct {
		Contents []struct {
			Key string `xml:"Key"`
		} `xml:"Contents"`
	}
	if decErr := xml.Unmarshal(body, &result); decErr != nil {
		t.Fatalf("decode prefix result: %v", decErr)
	}
	if len(result.Contents) != 2 {
		t.Errorf("ListObjectsV2 prefix=a/ count = %d; want 2", len(result.Contents))
	}
}

func listWithDelimiterV2(t *testing.T, bucket string) {
	t.Helper()
	resp, err := doSignedRequest(http.MethodGet, s3URL(bucket, "")+"?list-type=2&delimiter=%2F", nil, nil)
	if err != nil {
		t.Fatalf("ListObjectsV2 delimiter: %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()

	var result struct {
		CommonPrefixes []struct {
			Prefix string `xml:"Prefix"`
		} `xml:"CommonPrefixes"`
	}
	if decErr := xml.Unmarshal(body, &result); decErr != nil {
		t.Fatalf("decode delimiter result: %v", decErr)
	}
	if len(result.CommonPrefixes) != 2 {
		t.Errorf("ListObjectsV2 delimiter=/ CommonPrefixes count = %d; want 2", len(result.CommonPrefixes))
	}
}

// TestMinioMultipartUpload verifies 3-part multipart upload end-to-end.
func TestMinioMultipartUpload(t *testing.T) {
	bucket := fmt.Sprintf("minio-mp-%d", time.Now().UnixNano())
	createBucket(t, bucket)
	t.Cleanup(func() { cleanupBucket(t, bucket, "mp-key") })

	// Part data (3 parts of ~1KB each for test speed).
	part1 := bytes.Repeat([]byte("A"), 1024)
	part2 := bytes.Repeat([]byte("B"), 1024)
	part3 := bytes.Repeat([]byte("C"), 512)

	uploadID := initiateMultipart(t, bucket, "mp-key")

	etag1 := uploadOnePart(t, bucket, "mp-key", uploadID, 1, part1)
	etag2 := uploadOnePart(t, bucket, "mp-key", uploadID, 2, part2)
	etag3 := uploadOnePart(t, bucket, "mp-key", uploadID, 3, part3)

	completeMultipart(t, bucket, "mp-key", uploadID, etag1, etag2, etag3)

	// GetObject — verify assembled content.
	resp, err := doSignedRequest(http.MethodGet, s3URL(bucket, "mp-key"), nil, nil)
	if err != nil {
		t.Fatalf("GetObject after multipart: %v", err)
	}
	got, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GetObject after multipart status = %d; want 200", resp.StatusCode)
	}
	want := append(append(part1, part2...), part3...)
	if !bytes.Equal(got, want) {
		t.Errorf("multipart assembled content length mismatch: got %d bytes; want %d bytes", len(got), len(want))
	}
}

func initiateMultipart(t *testing.T, bucket, key string) string {
	t.Helper()
	resp, err := doSignedRequest(http.MethodPost, s3URL(bucket, key)+"?uploads", nil,
		map[string]string{"Content-Type": "application/octet-stream"})
	if err != nil {
		t.Fatalf("CreateMultipartUpload: %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("CreateMultipartUpload status = %d body = %s; want 200", resp.StatusCode, body)
	}
	var result struct {
		UploadID string `xml:"UploadId"`
	}
	if decErr := xml.Unmarshal(body, &result); decErr != nil {
		t.Fatalf("decode CreateMultipartUpload: %v", decErr)
	}
	if result.UploadID == "" {
		t.Fatal("CreateMultipartUpload: empty UploadId")
	}
	return result.UploadID
}

func uploadOnePart(t *testing.T, bucket, key, uploadID string, partNum int, data []byte) string {
	t.Helper()
	url := fmt.Sprintf("%s?partNumber=%d&uploadId=%s", s3URL(bucket, key), partNum, uploadID)
	r, err := doSignedRequest(http.MethodPut, url, data, nil)
	if err != nil {
		t.Fatalf("UploadPart %d: %v", partNum, err)
	}
	_ = r.Body.Close()
	if r.StatusCode != http.StatusOK {
		t.Fatalf("UploadPart %d status = %d; want 200", partNum, r.StatusCode)
	}
	etag := r.Header.Get("ETag")
	if etag == "" {
		t.Fatalf("UploadPart %d: missing ETag", partNum)
	}
	return etag
}

func completeMultipart(t *testing.T, bucket, key, uploadID string, etags ...string) {
	t.Helper()
	var sb strings.Builder
	sb.WriteString("<CompleteMultipartUpload>")
	for i, etag := range etags {
		fmt.Fprintf(&sb, "<Part><PartNumber>%d</PartNumber><ETag>%s</ETag></Part>", i+1, etag)
	}
	sb.WriteString("</CompleteMultipartUpload>")

	url := fmt.Sprintf("%s?uploadId=%s", s3URL(bucket, key), uploadID)
	resp, err := doSignedRequest(http.MethodPost, url, []byte(sb.String()),
		map[string]string{"Content-Type": "application/xml"})
	if err != nil {
		t.Fatalf("CompleteMultipartUpload: %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("CompleteMultipartUpload status = %d body = %s; want 200", resp.StatusCode, body)
	}
	if !bytes.Contains(body, []byte("CompleteMultipartUploadResult")) {
		t.Errorf("CompleteMultipartUpload: missing expected XML element; body = %s", body)
	}
}

// TestMinioDeleteObjects verifies batch DeleteObjects.
func TestMinioDeleteObjects(t *testing.T) {
	bucket := fmt.Sprintf("minio-batch-%d", time.Now().UnixNano())
	createBucket(t, bucket)
	t.Cleanup(func() { cleanupBucket(t, bucket, "del3") })

	for _, key := range []string{"del1", "del2", "del3"} {
		putObject(t, bucket, key, "content")
	}

	reqBody := []byte(`<Delete><Object><Key>del1</Key></Object><Object><Key>del2</Key></Object></Delete>`)
	resp, err := doSignedRequest(http.MethodPost, s3URL(bucket, "")+"?delete", reqBody,
		map[string]string{"Content-Type": "application/xml"})
	if err != nil {
		t.Fatalf("DeleteObjects: %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("DeleteObjects status = %d body = %s; want 200", resp.StatusCode, body)
	}

	var result struct {
		XMLName xml.Name `xml:"DeleteResult"`
		Deleted []struct {
			Key string `xml:"Key"`
		} `xml:"Deleted"`
	}
	if decErr := xml.Unmarshal(body, &result); decErr != nil {
		t.Fatalf("decode DeleteResult: %v", decErr)
	}
	if len(result.Deleted) != 2 {
		t.Errorf("DeleteObjects: got %d deleted; want 2; body = %s", len(result.Deleted), body)
	}

	// del3 should still exist.
	resp2, err := doSignedRequest(http.MethodHead, s3URL(bucket, "del3"), nil, nil)
	if err != nil {
		t.Fatalf("HeadObject del3: %v", err)
	}
	_ = resp2.Body.Close()
	if resp2.StatusCode != http.StatusOK {
		t.Errorf("HeadObject del3 after batch delete status = %d; want 200", resp2.StatusCode)
	}
}

// TestMinioListBuckets verifies ListBuckets (GET /).
func TestMinioListBuckets(t *testing.T) {
	bucket := fmt.Sprintf("minio-lb-%d", time.Now().UnixNano())
	createBucket(t, bucket)
	t.Cleanup(func() { cleanupBucket(t, bucket) })

	resp, err := doSignedRequest(http.MethodGet, s3Endpoint+"/", nil, nil)
	if err != nil {
		t.Fatalf("ListBuckets: %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("ListBuckets status = %d; want 200", resp.StatusCode)
	}
	if !bytes.Contains(body, []byte("ListAllMyBucketsResult")) {
		t.Errorf("ListBuckets response does not contain expected XML element; body = %s", body)
	}
	if !bytes.Contains(body, []byte(bucket)) {
		t.Errorf("ListBuckets response does not contain created bucket %q", bucket)
	}
}

// TestMinioDeleteBucket verifies DeleteBucket (empty and non-empty).
func TestMinioDeleteBucket(t *testing.T) {
	bucket := fmt.Sprintf("minio-delbkt-%d", time.Now().UnixNano())
	createBucket(t, bucket)

	// DeleteBucket — empty bucket.
	resp, err := doSignedRequest(http.MethodDelete, s3URL(bucket, ""), nil, nil)
	if err != nil {
		t.Fatalf("DeleteBucket empty: %v", err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Errorf("DeleteBucket empty status = %d; want 204", resp.StatusCode)
	}

	// HeadBucket on deleted bucket should 404.
	resp2, err := doSignedRequest(http.MethodHead, s3URL(bucket, ""), nil, nil)
	if err != nil {
		t.Fatalf("HeadBucket after delete: %v", err)
	}
	_ = resp2.Body.Close()
	if resp2.StatusCode != http.StatusNotFound {
		t.Errorf("HeadBucket after delete status = %d; want 404", resp2.StatusCode)
	}

	// DeleteBucket — non-empty should return 409.
	bucket2 := fmt.Sprintf("minio-nonempty-%d", time.Now().UnixNano())
	createBucket(t, bucket2)
	putObject(t, bucket2, "k", "v")
	t.Cleanup(func() { cleanupBucket(t, bucket2, "k") })

	resp3, err := doSignedRequest(http.MethodDelete, s3URL(bucket2, ""), nil, nil)
	if err != nil {
		t.Fatalf("DeleteBucket non-empty: %v", err)
	}
	_ = resp3.Body.Close()
	if resp3.StatusCode != http.StatusConflict {
		t.Errorf("DeleteBucket non-empty status = %d; want 409", resp3.StatusCode)
	}
}

// cleanupBucket deletes objects by key then deletes the bucket (best-effort).
func cleanupBucket(t *testing.T, bucket string, keys ...string) {
	t.Helper()
	for _, k := range keys {
		r, _ := doSignedRequest(http.MethodDelete, s3URL(bucket, k), nil, nil)
		if r != nil {
			_ = r.Body.Close()
		}
	}
	r, _ := doSignedRequest(http.MethodDelete, s3URL(bucket, ""), nil, nil)
	if r != nil {
		_ = r.Body.Close()
	}
}
