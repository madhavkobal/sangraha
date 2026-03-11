package integration

import (
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

const versioningEnabledXML = `<VersioningConfiguration xmlns="http://s3.amazonaws.com/doc/2006-03-01/"><Status>Enabled</Status></VersioningConfiguration>`

// enableVersioning sets versioning=Enabled on bucket via PUT /{bucket}?versioning.
func enableVersioning(t *testing.T, bucket string) {
	t.Helper()
	resp, err := doSignedRequest(http.MethodPut,
		s3URL(bucket, "")+"?versioning",
		[]byte(versioningEnabledXML),
		map[string]string{"Content-Type": "application/xml"},
	)
	if err != nil {
		t.Fatalf("enable versioning on %s: %v", bucket, err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("enable versioning on %s: want 200 got %d", bucket, resp.StatusCode)
	}
}

// putVersion puts an object and returns the version ID from the response header.
func putVersion(t *testing.T, bucket, key string, body []byte) string {
	t.Helper()
	resp, err := doSignedRequest(http.MethodPut, s3URL(bucket, key), body, nil)
	if err != nil {
		t.Fatalf("put %s/%s: %v", bucket, key, err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("put %s/%s: want 200 got %d", bucket, key, resp.StatusCode)
	}
	vid := resp.Header.Get("X-Amz-Version-Id")
	if vid == "" {
		t.Fatalf("put %s/%s: missing x-amz-version-id", bucket, key)
	}
	return vid
}

// listVersionsBody calls GET /{bucket}?versions and returns the raw XML body.
func listVersionsBody(t *testing.T, bucket string) []byte {
	t.Helper()
	resp, err := doSignedRequest(http.MethodGet, s3URL(bucket, "")+"?versions", nil, nil)
	if err != nil {
		t.Fatalf("list versions %s: %v", bucket, err)
	}
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("list versions %s: want 200 got %d: %s", bucket, resp.StatusCode, body)
	}
	return body
}

// getObjectVersion fetches a specific version and returns its body.
func getObjectVersion(t *testing.T, bucket, key, vid string) []byte {
	t.Helper()
	resp, err := doSignedRequest(http.MethodGet, s3URL(bucket, key)+"?versionId="+vid, nil, nil)
	if err != nil {
		t.Fatalf("get %s/%s@%s: %v", bucket, key, vid, err)
	}
	data, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("get %s/%s@%s: want 200 got %d", bucket, key, vid, resp.StatusCode)
	}
	return data
}

// TestVersioningEnableAndPut enables versioning on a bucket, puts two objects
// with the same key, and verifies that both version IDs are distinct.
func TestVersioningEnableAndPut(t *testing.T) {
	if rootAK == "" {
		t.Skip("SANGRAHA_ACCESS_KEY not set")
	}
	bucket := "it-versioning-" + fmt.Sprintf("%d", time.Now().UnixNano())
	defer deleteBucketFully(t, bucket)

	createBucket(t, bucket)
	enableVersioning(t, bucket)

	key := "versioned-object.txt"
	vid0 := putVersion(t, bucket, key, []byte("content version 0"))
	vid1 := putVersion(t, bucket, key, []byte("content version 1"))

	if vid0 == vid1 {
		t.Errorf("expected distinct version IDs, both got %q", vid0)
	}

	assertListVersionsCount(t, bucket, 2)

	got0 := getObjectVersion(t, bucket, key, vid0)
	if string(got0) != "content version 0" {
		t.Errorf("version 0 content = %q; want %q", got0, "content version 0")
	}
	got1 := getObjectVersion(t, bucket, key, vid1)
	if string(got1) != "content version 1" {
		t.Errorf("version 1 content = %q; want %q", got1, "content version 1")
	}
}

// assertListVersionsCount asserts that ListObjectVersions returns at least n versions.
func assertListVersionsCount(t *testing.T, bucket string, n int) {
	t.Helper()
	body := listVersionsBody(t, bucket)
	var result struct {
		Versions []struct {
			Key       string `xml:"Key"`
			VersionID string `xml:"VersionId"`
		} `xml:"Version"`
	}
	if err := xml.Unmarshal(body, &result); err != nil {
		t.Fatalf("list versions: unmarshal: %v", err)
	}
	if len(result.Versions) < n {
		t.Errorf("expected ≥ %d versions, got %d", n, len(result.Versions))
	}
}

// TestVersioningDeleteMarker verifies that deleting a versioned object without
// a version ID creates a delete marker.
func TestVersioningDeleteMarker(t *testing.T) {
	if rootAK == "" {
		t.Skip("SANGRAHA_ACCESS_KEY not set")
	}
	bucket := "it-delmk-" + fmt.Sprintf("%d", time.Now().UnixNano())
	defer deleteBucketFully(t, bucket)

	createBucket(t, bucket)
	enableVersioning(t, bucket)

	key := "dm-key.txt"
	putVersion(t, bucket, key, []byte("hello"))

	// Delete without version ID — should create delete marker.
	dmVersionID := deleteAndGetMarkerID(t, bucket, key)
	if dmVersionID == "" {
		t.Fatal("delete response missing x-amz-version-id (delete marker)")
	}

	assertObjectNotFound(t, bucket, key)
	assertListVersionsHasDeleteMarker(t, bucket)
}

// deleteAndGetMarkerID deletes an object and returns the delete marker version ID.
func deleteAndGetMarkerID(t *testing.T, bucket, key string) string {
	t.Helper()
	resp, err := doSignedRequest(http.MethodDelete, s3URL(bucket, key), nil, nil)
	if err != nil {
		t.Fatalf("delete %s/%s: %v", bucket, key, err)
	}
	_ = resp.Body.Close()
	return resp.Header.Get("X-Amz-Version-Id")
}

// assertObjectNotFound checks that GET /{bucket}/{key} returns 404.
func assertObjectNotFound(t *testing.T, bucket, key string) {
	t.Helper()
	resp, err := doSignedRequest(http.MethodGet, s3URL(bucket, key), nil, nil)
	if err != nil {
		t.Fatalf("get %s/%s: %v", bucket, key, err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("get after delete marker: want 404 got %d", resp.StatusCode)
	}
}

// assertListVersionsHasDeleteMarker asserts that ListObjectVersions contains a DeleteMarker element.
func assertListVersionsHasDeleteMarker(t *testing.T, bucket string) {
	t.Helper()
	body := listVersionsBody(t, bucket)
	if !strings.Contains(string(body), "DeleteMarker") {
		t.Errorf("list versions: expected DeleteMarker in response, body=%s", body)
	}
}

// TestVersioningDeleteSpecificVersion permanently deletes a specific version.
func TestVersioningDeleteSpecificVersion(t *testing.T) {
	if rootAK == "" {
		t.Skip("SANGRAHA_ACCESS_KEY not set")
	}
	bucket := "it-delver-" + fmt.Sprintf("%d", time.Now().UnixNano())
	defer deleteBucketFully(t, bucket)

	createBucket(t, bucket)
	enableVersioning(t, bucket)

	key := "perm-del.txt"
	vid := putVersion(t, bucket, key, []byte("data"))

	deleteSpecificVersion(t, bucket, key, vid)
	assertVersionNotFound(t, bucket, key, vid)
}

// deleteSpecificVersion calls DELETE /{bucket}/{key}?versionId={vid}.
func deleteSpecificVersion(t *testing.T, bucket, key, vid string) {
	t.Helper()
	resp, err := doSignedRequest(http.MethodDelete, s3URL(bucket, key)+"?versionId="+vid, nil, nil)
	if err != nil {
		t.Fatalf("delete version %s/%s@%s: %v", bucket, key, vid, err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Errorf("delete version: want 204 got %d", resp.StatusCode)
	}
}

// assertVersionNotFound checks that GET /{bucket}/{key}?versionId={vid} returns 404.
func assertVersionNotFound(t *testing.T, bucket, key, vid string) {
	t.Helper()
	resp, err := doSignedRequest(http.MethodGet, s3URL(bucket, key)+"?versionId="+vid, nil, nil)
	if err != nil {
		t.Fatalf("get version %s/%s@%s: %v", bucket, key, vid, err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("get deleted version: want 404 got %d", resp.StatusCode)
	}
}

// deleteBucketFully removes all versions and delete markers, then deletes the bucket.
func deleteBucketFully(t *testing.T, bucket string) {
	t.Helper()
	purgeAllVersions(t, bucket)
	resp, _ := doSignedRequest(http.MethodDelete, s3URL(bucket, ""), nil, nil)
	if resp != nil {
		_ = resp.Body.Close()
	}
}

// purgeAllVersions lists and permanently deletes all versions and delete markers.
func purgeAllVersions(t *testing.T, bucket string) {
	t.Helper()
	resp, err := doSignedRequest(http.MethodGet, s3URL(bucket, "")+"?versions", nil, nil)
	if err != nil || resp.StatusCode != http.StatusOK {
		if resp != nil {
			_ = resp.Body.Close()
		}
		return
	}
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()

	var vlist struct {
		Versions []struct {
			Key       string `xml:"Key"`
			VersionID string `xml:"VersionId"`
		} `xml:"Version"`
		DeleteMarkers []struct {
			Key       string `xml:"Key"`
			VersionID string `xml:"VersionId"`
		} `xml:"DeleteMarker"`
	}
	_ = xml.Unmarshal(body, &vlist)
	for _, v := range vlist.Versions {
		r, _ := doSignedRequest(http.MethodDelete, s3URL(bucket, v.Key)+"?versionId="+v.VersionID, nil, nil)
		if r != nil {
			_ = r.Body.Close()
		}
	}
	for _, dm := range vlist.DeleteMarkers {
		r, _ := doSignedRequest(http.MethodDelete, s3URL(bucket, dm.Key)+"?versionId="+dm.VersionID, nil, nil)
		if r != nil {
			_ = r.Body.Close()
		}
	}
}
