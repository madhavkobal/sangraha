package storage

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/madhavkobal/sangraha/internal/backend/localfs"
	metabbolt "github.com/madhavkobal/sangraha/internal/metadata/bbolt"
)

func newTestEngine(t *testing.T) *Engine {
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
	return New(be, meta, "root")
}

func TestCreateAndHeadBucket(t *testing.T) {
	e := newTestEngine(t)
	ctx := context.Background()

	if err := e.CreateBucket(ctx, "my-bucket", "root", "us-east-1"); err != nil {
		t.Fatalf("CreateBucket: %v", err)
	}
	rec, err := e.HeadBucket(ctx, "my-bucket")
	if err != nil {
		t.Fatalf("HeadBucket: %v", err)
	}
	if rec.Name != "my-bucket" {
		t.Errorf("HeadBucket.Name = %q; want my-bucket", rec.Name)
	}
}

func TestCreateBucketAlreadyExists(t *testing.T) {
	e := newTestEngine(t)
	ctx := context.Background()

	_ = e.CreateBucket(ctx, "dup-bucket", "root", "")
	err := e.CreateBucket(ctx, "dup-bucket", "root", "")
	if _, ok := err.(*BucketAlreadyExistsError); !ok {
		t.Errorf("expected BucketAlreadyExistsError, got %T: %v", err, err)
	}
}

func TestDeleteBucket(t *testing.T) {
	e := newTestEngine(t)
	ctx := context.Background()

	_ = e.CreateBucket(ctx, "del-bucket", "root", "")
	if err := e.DeleteBucket(ctx, "del-bucket"); err != nil {
		t.Fatalf("DeleteBucket: %v", err)
	}
	if _, err := e.HeadBucket(ctx, "del-bucket"); err == nil {
		t.Error("expected error after delete, got nil")
	}
}

func TestDeleteBucketNotFound(t *testing.T) {
	e := newTestEngine(t)
	err := e.DeleteBucket(context.Background(), "ghost-bucket")
	if _, ok := err.(*BucketNotFoundError); !ok {
		t.Errorf("expected BucketNotFoundError, got %T: %v", err, err)
	}
}

func TestListBuckets(t *testing.T) {
	e := newTestEngine(t)
	ctx := context.Background()

	for _, name := range []string{"bucket-a", "bucket-b", "bucket-c"} {
		_ = e.CreateBucket(ctx, name, "root", "")
	}
	buckets, err := e.ListBuckets(ctx)
	if err != nil {
		t.Fatalf("ListBuckets: %v", err)
	}
	if len(buckets) != 3 {
		t.Errorf("ListBuckets len = %d; want 3", len(buckets))
	}
}

func TestValidateBucketName(t *testing.T) {
	tests := []struct {
		name  string
		valid bool
	}{
		{"valid-bucket", true},
		{"ab", false},                // too short
		{"UPPERCASE", false},         // uppercase
		{"with space", false},        // space
		{"has..dots", false},         // double dot not caught by regex but caught by prefix check
		{"-starts-with-dash", false}, // starts with dash
		{"ends-with-dash-", false},   // ends with dash
	}
	for _, tc := range tests {
		err := validateBucketName(tc.name)
		got := err == nil
		if got != tc.valid {
			t.Errorf("validateBucketName(%q) valid=%v; want %v (err: %v)", tc.name, got, tc.valid, err)
		}
	}
}

func TestErrorMessages(t *testing.T) {
	e1 := &BucketAlreadyExistsError{Name: "my-bucket"}
	if e1.Error() == "" {
		t.Error("BucketAlreadyExistsError.Error() should not be empty")
	}

	e2 := &BucketNotFoundError{Name: "missing"}
	if e2.Error() == "" {
		t.Error("BucketNotFoundError.Error() should not be empty")
	}

	e3 := &BucketNotEmptyError{Name: "nonempty"}
	if e3.Error() == "" {
		t.Error("BucketNotEmptyError.Error() should not be empty")
	}
}
