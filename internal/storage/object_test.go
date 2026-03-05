package storage

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/madhavkobal/sangraha/internal/backend"
	"github.com/madhavkobal/sangraha/internal/metadata"
)

func TestPutAndGetObject(t *testing.T) {
	e := newTestEngine(t)
	ctx := context.Background()

	_ = e.CreateBucket(ctx, "bucket", "root", "")
	body := strings.NewReader("hello world")
	out, err := e.PutObject(ctx, PutObjectInput{
		Bucket: "bucket",
		Key:    "test/key",
		Body:   body,
		Size:   11,
		Owner:  "root",
	})
	if err != nil {
		t.Fatalf("PutObject: %v", err)
	}
	if out.ETag == "" {
		t.Error("PutObject ETag is empty")
	}
	// ETag must be quoted.
	if out.ETag[0] != '"' || out.ETag[len(out.ETag)-1] != '"' {
		t.Errorf("ETag %q is not quoted", out.ETag)
	}

	getOut, err := e.GetObject(ctx, GetObjectInput{Bucket: "bucket", Key: "test/key"})
	if err != nil {
		t.Fatalf("GetObject: %v", err)
	}
	defer func() { _ = getOut.Body.Close() }()
	data, _ := io.ReadAll(getOut.Body)
	if string(data) != "hello world" {
		t.Errorf("GetObject body = %q; want %q", data, "hello world")
	}
	if getOut.Record.Size != 11 {
		t.Errorf("GetObject Size = %d; want 11", getOut.Record.Size)
	}
}

func TestHeadObject(t *testing.T) {
	e := newTestEngine(t)
	ctx := context.Background()

	_ = e.CreateBucket(ctx, "bucket", "root", "")
	_, _ = e.PutObject(ctx, PutObjectInput{
		Bucket: "bucket", Key: "mykey",
		Body: strings.NewReader("data"), Size: 4, Owner: "root",
	})

	rec, err := e.HeadObject(ctx, "bucket", "mykey")
	if err != nil {
		t.Fatalf("HeadObject: %v", err)
	}
	if rec.Key != "mykey" {
		t.Errorf("HeadObject.Key = %q; want mykey", rec.Key)
	}
}

func TestDeleteObject(t *testing.T) {
	e := newTestEngine(t)
	ctx := context.Background()

	_ = e.CreateBucket(ctx, "bucket", "root", "")
	_, _ = e.PutObject(ctx, PutObjectInput{
		Bucket: "bucket", Key: "del-key",
		Body: strings.NewReader("x"), Size: 1, Owner: "root",
	})
	if err := e.DeleteObject(ctx, "bucket", "del-key"); err != nil {
		t.Fatalf("DeleteObject: %v", err)
	}
	if _, err := e.HeadObject(ctx, "bucket", "del-key"); err == nil {
		t.Error("object still exists after delete")
	}
	// Delete again should be idempotent.
	if err := e.DeleteObject(ctx, "bucket", "del-key"); err != nil {
		t.Errorf("Delete (idempotent): %v", err)
	}
}

func TestCopyObject(t *testing.T) {
	e := newTestEngine(t)
	ctx := context.Background()

	_ = e.CreateBucket(ctx, "src-bucket", "root", "")
	_ = e.CreateBucket(ctx, "dst-bucket", "root", "")
	_, _ = e.PutObject(ctx, PutObjectInput{
		Bucket: "src-bucket", Key: "original",
		Body: strings.NewReader("copy-me"), Size: 7, Owner: "root",
	})

	rec, err := e.CopyObject(ctx, "src-bucket", "original", "dst-bucket", "copy", "root")
	if err != nil {
		t.Fatalf("CopyObject: %v", err)
	}
	if rec.Key != "copy" {
		t.Errorf("CopyObject.Key = %q; want copy", rec.Key)
	}

	getOut, _ := e.GetObject(ctx, GetObjectInput{Bucket: "dst-bucket", Key: "copy"})
	defer func() { _ = getOut.Body.Close() }()
	data, _ := io.ReadAll(getOut.Body)
	if string(data) != "copy-me" {
		t.Errorf("copied content = %q; want copy-me", data)
	}
}

func TestListObjects(t *testing.T) {
	e := newTestEngine(t)
	ctx := context.Background()

	_ = e.CreateBucket(ctx, "list-bucket", "root", "")
	for _, key := range []string{"a/1", "a/2", "b/1", "c"} {
		_, _ = e.PutObject(ctx, PutObjectInput{
			Bucket: "list-bucket", Key: key,
			Body: strings.NewReader("x"), Size: 1, Owner: "root",
		})
	}

	t.Run("no prefix", func(t *testing.T) {
		recs, _, err := e.ListObjects(ctx, "list-bucket", newListOpts("", "", 100))
		if err != nil {
			t.Fatalf("ListObjects: %v", err)
		}
		if len(recs) != 4 {
			t.Errorf("len = %d; want 4", len(recs))
		}
	})

	t.Run("prefix a/", func(t *testing.T) {
		recs, _, err := e.ListObjects(ctx, "list-bucket", newListOpts("a/", "", 100))
		if err != nil {
			t.Fatalf("ListObjects: %v", err)
		}
		if len(recs) != 2 {
			t.Errorf("len = %d; want 2", len(recs))
		}
	})

	t.Run("delimiter /", func(t *testing.T) {
		recs, prefixes, err := e.ListObjects(ctx, "list-bucket", newListOpts("", "/", 100))
		if err != nil {
			t.Fatalf("ListObjects: %v", err)
		}
		// Only "c" should appear as an object; a/ and b/ as common prefixes.
		if len(recs) != 1 {
			t.Errorf("records len = %d; want 1", len(recs))
		}
		if len(prefixes) != 2 {
			t.Errorf("prefixes len = %d; want 2", len(prefixes))
		}
	})
}

func newListOpts(prefix, delimiter string, max int) metadata.ListOptions {
	return metadata.ListOptions{Prefix: prefix, Delimiter: delimiter, MaxKeys: max}
}

func TestListObjectsNonExistentBucket(t *testing.T) {
	e := newTestEngine(t)
	_, _, err := e.ListObjects(context.Background(), "no-such-bucket", newListOpts("", "", 10))
	if err == nil {
		t.Fatal("expected error for missing bucket, got nil")
	}
}

func TestIsBackendNotFound(t *testing.T) {
	err := &backend.ErrNotFound{Bucket: "b", Key: "k"}
	if !isBackendNotFound(err) {
		t.Error("isBackendNotFound should return true for *backend.ErrNotFound")
	}
	if isBackendNotFound(fmt.Errorf("other error")) {
		t.Error("isBackendNotFound should return false for other errors")
	}
	if isBackendNotFound(nil) {
		t.Error("isBackendNotFound should return false for nil")
	}
}

func TestNewMD5(t *testing.T) {
	// MD5("") = d41d8cd98f00b204e9800998ecf8427e
	got := newMD5([]byte(""))
	want := "d41d8cd98f00b204e9800998ecf8427e"
	if got != want {
		t.Errorf("newMD5(\"\") = %q; want %q", got, want)
	}
	// MD5("hello") = 5d41402abc4b2a76b9719d911017c592
	got = newMD5([]byte("hello"))
	want = "5d41402abc4b2a76b9719d911017c592"
	if got != want {
		t.Errorf("newMD5(\"hello\") = %q; want %q", got, want)
	}
}

func TestDetectContentTypeEmpty(t *testing.T) {
	ct := detectContentType(nil)
	if ct != "application/octet-stream" {
		t.Errorf("detectContentType(nil) = %q; want application/octet-stream", ct)
	}
	ct = detectContentType([]byte{})
	if ct != "application/octet-stream" {
		t.Errorf("detectContentType([]) = %q; want application/octet-stream", ct)
	}
}

func TestDeleteObjects(t *testing.T) {
	e := newTestEngine(t)
	ctx := context.Background()

	_ = e.CreateBucket(ctx, "bucket", "root", "")
	for _, k := range []string{"k1", "k2", "k3"} {
		_, _ = e.PutObject(ctx, PutObjectInput{
			Bucket: "bucket", Key: k,
			Body: strings.NewReader("x"), Size: 1, Owner: "root",
		})
	}
	deleted, errs := e.DeleteObjects(ctx, "bucket", []string{"k1", "k2"})
	if len(deleted) != 2 {
		t.Errorf("deleted = %d; want 2", len(deleted))
	}
	if len(errs) != 0 {
		t.Errorf("errs = %v; want none", errs)
	}
}

func TestETagComputation(t *testing.T) {
	// Verify that the MD5 ETag for a known payload is computed correctly.
	// echo -n "hello" | md5sum == 5d41402abc4b2a76b9719d911017c592
	e := newTestEngine(t)
	ctx := context.Background()
	_ = e.CreateBucket(ctx, "bucket", "root", "")

	out, err := e.PutObject(ctx, PutObjectInput{
		Bucket: "bucket", Key: "k",
		Body: bytes.NewReader([]byte("hello")), Size: 5, Owner: "root",
	})
	if err != nil {
		t.Fatalf("PutObject: %v", err)
	}
	want := `"5d41402abc4b2a76b9719d911017c592"`
	if out.ETag != want {
		t.Errorf("ETag = %q; want %q", out.ETag, want)
	}
}

func TestObjectNotFoundError(t *testing.T) {
	e := &ObjectNotFoundError{Bucket: "b", Key: "k"}
	if e.Error() == "" {
		t.Error("ObjectNotFoundError.Error() should not be empty")
	}
}

func TestListObjectsErrors(t *testing.T) {
	e := newTestEngine(t)
	ctx := context.Background()

	_ = e.CreateBucket(ctx, "list-extra", "root", "")
	for _, k := range []string{"prefix/a", "prefix/b", "other"} {
		_, _ = e.PutObject(ctx, PutObjectInput{
			Bucket: "list-extra", Key: k,
			Body: strings.NewReader("x"), Size: 1, Owner: "root",
		})
	}

	// Prefix filter.
	recs, _, err := e.ListObjects(ctx, "list-extra", metadata.ListOptions{Prefix: "prefix/", MaxKeys: 10})
	if err != nil {
		t.Fatalf("ListObjects with prefix: %v", err)
	}
	if len(recs) != 2 {
		t.Errorf("got %d recs with prefix; want 2", len(recs))
	}

	// Delimiter.
	_, prefixes, err := e.ListObjects(ctx, "list-extra", metadata.ListOptions{Delimiter: "/", MaxKeys: 10})
	if err != nil {
		t.Fatalf("ListObjects with delimiter: %v", err)
	}
	if len(prefixes) != 1 {
		t.Errorf("got %d prefixes; want 1", len(prefixes))
	}
}
