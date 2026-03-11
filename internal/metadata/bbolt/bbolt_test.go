package bbolt

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/madhavkobal/sangraha/internal/metadata"
)

func openTestStore(t *testing.T) *Store {
	t.Helper()
	dir := t.TempDir()
	s, err := Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func TestBucketCRUD(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	rec := metadata.BucketRecord{
		Name:      "test-bucket",
		CreatedAt: time.Now().UTC().Truncate(time.Second),
		Owner:     "root",
		Region:    "us-east-1",
	}

	// Put and Get
	if err := s.PutBucket(ctx, rec); err != nil {
		t.Fatalf("PutBucket: %v", err)
	}
	got, err := s.GetBucket(ctx, "test-bucket")
	if err != nil {
		t.Fatalf("GetBucket: %v", err)
	}
	if got.Name != rec.Name || got.Owner != rec.Owner {
		t.Errorf("got %+v; want %+v", got, rec)
	}

	// BucketExists
	exists, err := s.BucketExists(ctx, "test-bucket")
	if err != nil || !exists {
		t.Errorf("BucketExists: got (%v, %v); want (true, nil)", exists, err)
	}
	exists, err = s.BucketExists(ctx, "missing")
	if err != nil || exists {
		t.Errorf("BucketExists missing: got (%v, %v); want (false, nil)", exists, err)
	}

	// ListBuckets
	_ = s.PutBucket(ctx, metadata.BucketRecord{Name: "alpha"})
	_ = s.PutBucket(ctx, metadata.BucketRecord{Name: "zeta"})
	buckets, err := s.ListBuckets(ctx)
	if err != nil {
		t.Fatalf("ListBuckets: %v", err)
	}
	if buckets[0].Name > buckets[1].Name {
		t.Error("ListBuckets should return buckets in lexicographic order")
	}

	// Delete
	err = s.DeleteBucket(ctx, "test-bucket")
	if err != nil {
		t.Fatalf("DeleteBucket: %v", err)
	}
	_, err = s.GetBucket(ctx, "test-bucket")
	if err == nil {
		t.Error("GetBucket after delete should return error")
	}
}

func TestBucketStats(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	_ = s.PutBucket(ctx, metadata.BucketRecord{Name: "stats-bucket"})

	if err := s.UpdateBucketStats(ctx, "stats-bucket", 3, 1024); err != nil {
		t.Fatalf("UpdateBucketStats: %v", err)
	}
	got, _ := s.GetBucket(ctx, "stats-bucket")
	if got.ObjectCount != 3 || got.TotalBytes != 1024 {
		t.Errorf("stats: got count=%d bytes=%d; want 3, 1024", got.ObjectCount, got.TotalBytes)
	}

	// Decrement
	if err := s.UpdateBucketStats(ctx, "stats-bucket", -1, -100); err != nil {
		t.Fatalf("UpdateBucketStats decrement: %v", err)
	}
	got, _ = s.GetBucket(ctx, "stats-bucket")
	if got.ObjectCount != 2 || got.TotalBytes != 924 {
		t.Errorf("after decrement: got count=%d bytes=%d; want 2, 924", got.ObjectCount, got.TotalBytes)
	}
}

func TestObjectCRUD(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	obj := metadata.ObjectRecord{
		Bucket:      "my-bucket",
		Key:         "path/to/file.txt",
		Size:        42,
		ETag:        `"abc123"`,
		ContentType: "text/plain",
		Owner:       "alice",
	}
	if err := s.PutObject(ctx, obj); err != nil {
		t.Fatalf("PutObject: %v", err)
	}

	got, err := s.GetObject(ctx, "my-bucket", "path/to/file.txt")
	if err != nil {
		t.Fatalf("GetObject: %v", err)
	}
	if got.Key != obj.Key || got.ETag != obj.ETag {
		t.Errorf("got %+v; want %+v", got, obj)
	}

	exists, _ := s.ObjectExists(ctx, "my-bucket", "path/to/file.txt")
	if !exists {
		t.Error("ObjectExists should be true")
	}

	// Delete
	if err := s.DeleteObject(ctx, "my-bucket", "path/to/file.txt"); err != nil {
		t.Fatalf("DeleteObject: %v", err)
	}
	exists, _ = s.ObjectExists(ctx, "my-bucket", "path/to/file.txt")
	if exists {
		t.Error("ObjectExists after delete should be false")
	}
}

func TestListObjects(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	for _, key := range []string{"a/1", "a/2", "b/1", "c"} {
		_ = s.PutObject(ctx, metadata.ObjectRecord{Bucket: "b", Key: key})
	}

	t.Run("all", func(t *testing.T) {
		recs, prefixes, err := s.ListObjects(ctx, "b", metadata.ListOptions{MaxKeys: 100})
		if err != nil {
			t.Fatal(err)
		}
		if len(recs) != 4 {
			t.Errorf("got %d records; want 4", len(recs))
		}
		if len(prefixes) != 0 {
			t.Errorf("got %d prefixes; want 0", len(prefixes))
		}
	})

	t.Run("prefix", func(t *testing.T) {
		recs, _, err := s.ListObjects(ctx, "b", metadata.ListOptions{Prefix: "a/", MaxKeys: 100})
		if err != nil {
			t.Fatal(err)
		}
		if len(recs) != 2 {
			t.Errorf("got %d records; want 2", len(recs))
		}
	})

	t.Run("delimiter", func(t *testing.T) {
		recs, prefixes, err := s.ListObjects(ctx, "b", metadata.ListOptions{Delimiter: "/", MaxKeys: 100})
		if err != nil {
			t.Fatal(err)
		}
		// Only "c" is a top-level object; a/ and b/ are common prefixes.
		if len(recs) != 1 {
			t.Errorf("got %d records; want 1", len(recs))
		}
		if len(prefixes) != 2 {
			t.Errorf("got %d prefixes; want 2", len(prefixes))
		}
	})

	t.Run("max-keys", func(t *testing.T) {
		recs, _, err := s.ListObjects(ctx, "b", metadata.ListOptions{MaxKeys: 2})
		if err != nil {
			t.Fatal(err)
		}
		if len(recs) != 2 {
			t.Errorf("got %d records; want 2", len(recs))
		}
	})
}

func TestMultipartCRUD(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	m := metadata.MultipartRecord{
		UploadID:  "upload-1",
		Bucket:    "my-bucket",
		Key:       "large.bin",
		Owner:     "root",
		Initiated: time.Now().UTC(),
	}
	if err := s.PutMultipart(ctx, m); err != nil {
		t.Fatalf("PutMultipart: %v", err)
	}
	got, err := s.GetMultipart(ctx, "upload-1")
	if err != nil {
		t.Fatalf("GetMultipart: %v", err)
	}
	if got.Key != m.Key {
		t.Errorf("got key %q; want %q", got.Key, m.Key)
	}

	// Parts
	for i := 1; i <= 3; i++ {
		_ = s.PutPart(ctx, metadata.PartRecord{
			UploadID:   "upload-1",
			PartNumber: i,
			ETag:       `"part"`,
			Size:       1024,
		})
	}
	parts, err := s.ListParts(ctx, "upload-1")
	if err != nil {
		t.Fatalf("ListParts: %v", err)
	}
	if len(parts) != 3 {
		t.Errorf("got %d parts; want 3", len(parts))
	}
	// Verify ordering
	for i, p := range parts {
		if p.PartNumber != i+1 {
			t.Errorf("part[%d].PartNumber = %d; want %d", i, p.PartNumber, i+1)
		}
	}

	// DeleteParts
	err = s.DeleteParts(ctx, "upload-1")
	if err != nil {
		t.Fatalf("DeleteParts: %v", err)
	}
	parts, _ = s.ListParts(ctx, "upload-1")
	if len(parts) != 0 {
		t.Errorf("after DeleteParts, got %d parts; want 0", len(parts))
	}

	// ListMultiparts
	_ = s.PutMultipart(ctx, metadata.MultipartRecord{UploadID: "u2", Bucket: "my-bucket"})
	_ = s.PutMultipart(ctx, metadata.MultipartRecord{UploadID: "u3", Bucket: "other-bucket"})
	uploads, err := s.ListMultiparts(ctx, "my-bucket")
	if err != nil {
		t.Fatalf("ListMultiparts: %v", err)
	}
	if len(uploads) != 2 { // upload-1 and u2
		t.Errorf("got %d uploads; want 2", len(uploads))
	}

	// DeleteMultipart
	err = s.DeleteMultipart(ctx, "upload-1")
	if err != nil {
		t.Fatalf("DeleteMultipart: %v", err)
	}
	_, err = s.GetMultipart(ctx, "upload-1")
	if err == nil {
		t.Error("GetMultipart after delete should return error")
	}
}

func TestAccessKeyCRUD(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	rec := metadata.AccessKeyRecord{
		AccessKey:  "AKIATEST1234",
		SecretHash: "hashvalue",
		Owner:      "alice",
		IsRoot:     false,
		CreatedAt:  time.Now().UTC(),
	}
	if err := s.PutAccessKey(ctx, rec); err != nil {
		t.Fatalf("PutAccessKey: %v", err)
	}

	got, err := s.GetAccessKey(ctx, "AKIATEST1234")
	if err != nil {
		t.Fatalf("GetAccessKey: %v", err)
	}
	if got.Owner != "alice" {
		t.Errorf("owner = %q; want %q", got.Owner, "alice")
	}

	keys, err := s.ListAccessKeys(ctx)
	if err != nil {
		t.Fatalf("ListAccessKeys: %v", err)
	}
	if len(keys) != 1 {
		t.Errorf("got %d keys; want 1", len(keys))
	}

	err = s.DeleteAccessKey(ctx, "AKIATEST1234")
	if err != nil {
		t.Fatalf("DeleteAccessKey: %v", err)
	}
	_, err = s.GetAccessKey(ctx, "AKIATEST1234")
	if err == nil {
		t.Error("GetAccessKey after delete should return error")
	}
}

func TestOpenInvalidPath(t *testing.T) {
	_, err := Open(filepath.Join(os.DevNull, "test.db"))
	if err == nil {
		t.Error("Open on invalid path should return error")
	}
}
