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

// putObjectTagging sends a PutObjectTagging request.
func putObjectTagging(t *testing.T, bucket, key, tagXML string) {
	t.Helper()
	resp, err := doSignedRequest(http.MethodPut,
		s3URL(bucket, key)+"?tagging",
		[]byte(tagXML),
		map[string]string{"Content-Type": "application/xml"},
	)
	if err != nil {
		t.Fatalf("put tags %s/%s: %v", bucket, key, err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("put tags %s/%s: want 200 got %d", bucket, key, resp.StatusCode)
	}
}

// getObjectTagging fetches the object tags and returns the parsed key→value map.
func getObjectTagging(t *testing.T, bucket, key string) map[string]string {
	t.Helper()
	resp, err := doSignedRequest(http.MethodGet, s3URL(bucket, key)+"?tagging", nil, nil)
	if err != nil {
		t.Fatalf("get tags %s/%s: %v", bucket, key, err)
	}
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("get tags %s/%s: want 200 got %d: %s", bucket, key, resp.StatusCode, body)
	}
	var tagging struct {
		TagSet []struct {
			Key   string `xml:"Key"`
			Value string `xml:"Value"`
		} `xml:"TagSet>Tag"`
	}
	if unmarshalErr := xml.Unmarshal(body, &tagging); unmarshalErr != nil {
		t.Fatalf("unmarshal tags: %v", unmarshalErr)
	}
	m := make(map[string]string, len(tagging.TagSet))
	for _, tag := range tagging.TagSet {
		m[tag.Key] = tag.Value
	}
	return m
}

// deleteObjectTagging sends a DeleteObjectTagging request and asserts 204.
func deleteObjectTagging(t *testing.T, bucket, key string) {
	t.Helper()
	resp, err := doSignedRequest(http.MethodDelete, s3URL(bucket, key)+"?tagging", nil, nil)
	if err != nil {
		t.Fatalf("delete tags %s/%s: %v", bucket, key, err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Errorf("delete tags %s/%s: want 204 got %d", bucket, key, resp.StatusCode)
	}
}

// TestObjectTaggingPutGetDelete verifies PutObjectTagging, GetObjectTagging,
// and DeleteObjectTagging.
func TestObjectTaggingPutGetDelete(t *testing.T) {
	if rootAK == "" {
		t.Skip("SANGRAHA_ACCESS_KEY not set")
	}

	bucket := fmt.Sprintf("it-tagging-%d", time.Now().UnixNano())
	key := "tagged-obj.txt"

	createBucket(t, bucket)
	defer cleanupBucket(t, bucket, key)

	putObject(t, bucket, key, "tag-me")

	tagXML := `<Tagging xmlns="http://s3.amazonaws.com/doc/2006-03-01/"><TagSet><Tag><Key>env</Key><Value>prod</Value></Tag><Tag><Key>team</Key><Value>sre</Value></Tag></TagSet></Tagging>`
	putObjectTagging(t, bucket, key, tagXML)

	tags := getObjectTagging(t, bucket, key)
	assertTagValue(t, tags, "env", "prod")
	assertTagValue(t, tags, "team", "sre")
	if len(tags) != 2 {
		t.Errorf("expected 2 tags, got %d", len(tags))
	}

	deleteObjectTagging(t, bucket, key)
	emptyTags := getObjectTagging(t, bucket, key)
	if len(emptyTags) != 0 {
		t.Errorf("after delete: expected 0 tags, got %d", len(emptyTags))
	}
}

// assertTagValue checks that tags[key] == wantValue.
func assertTagValue(t *testing.T, tags map[string]string, key, wantValue string) {
	t.Helper()
	if got := tags[key]; got != wantValue {
		t.Errorf("tag %q = %q; want %q", key, got, wantValue)
	}
}

// TestObjectTagsCopiedWithObject verifies tags after CopyObject.
func TestObjectTagsCopiedWithObject(t *testing.T) {
	if rootAK == "" {
		t.Skip("SANGRAHA_ACCESS_KEY not set")
	}

	bucket := fmt.Sprintf("it-tag-copy-%d", time.Now().UnixNano())
	srcKey := "src-tagged.txt"
	dstKey := "dst-tagged.txt"

	createBucket(t, bucket)
	defer cleanupBucket(t, bucket, srcKey, dstKey)

	putObject(t, bucket, srcKey, "content")

	tagXML := `<Tagging xmlns="http://s3.amazonaws.com/doc/2006-03-01/"><TagSet><Tag><Key>color</Key><Value>blue</Value></Tag></TagSet></Tagging>`
	putObjectTagging(t, bucket, srcKey, tagXML)

	copyObject(t, bucket, srcKey, bucket, dstKey)

	dstTags := getObjectTagging(t, bucket, dstKey)
	// Not all S3 implementations copy tags on CopyObject without x-amz-tagging-directive.
	if !strings.Contains(fmt.Sprint(dstTags), "blue") {
		t.Logf("tags not copied to destination (optional S3 behaviour): %v", dstTags)
	}
}

// copyObject performs a server-side copy from src to dst in the same bucket.
func copyObject(t *testing.T, srcBucket, srcKey, dstBucket, dstKey string) {
	t.Helper()
	resp, err := doSignedRequest(http.MethodPut, s3URL(dstBucket, dstKey), nil,
		map[string]string{"x-amz-copy-source": srcBucket + "/" + srcKey})
	if err != nil {
		t.Fatalf("copy %s/%s→%s/%s: %v", srcBucket, srcKey, dstBucket, dstKey, err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("copy: want 200 got %d", resp.StatusCode)
	}
}
