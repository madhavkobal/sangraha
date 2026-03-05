package localfs

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestWriteRead(t *testing.T) {
	dir := t.TempDir()
	b, err := New(dir)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	ctx := context.Background()

	data := []byte("hello, sangraha!")
	n, err := b.Write(ctx, "mybucket", "mykey", bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("Write: %v", err)
	}
	if n != int64(len(data)) {
		t.Errorf("Write returned %d bytes; want %d", n, len(data))
	}

	var buf bytes.Buffer
	if err = b.Read(ctx, "mybucket", "mykey", &buf); err != nil {
		t.Fatalf("Read: %v", err)
	}
	if !bytes.Equal(buf.Bytes(), data) {
		t.Errorf("Read = %q; want %q", buf.Bytes(), data)
	}
}

func TestExists(t *testing.T) {
	dir := t.TempDir()
	b, _ := New(dir)
	ctx := context.Background()

	ok, err := b.Exists(ctx, "bucket", "key")
	if err != nil {
		t.Fatalf("Exists: %v", err)
	}
	if ok {
		t.Error("Exists returned true for non-existent object")
	}

	_, _ = b.Write(ctx, "bucket", "key", strings.NewReader("data"), 4)

	ok, err = b.Exists(ctx, "bucket", "key")
	if err != nil {
		t.Fatalf("Exists after write: %v", err)
	}
	if !ok {
		t.Error("Exists returned false after write")
	}
}

func TestStat(t *testing.T) {
	dir := t.TempDir()
	b, _ := New(dir)
	ctx := context.Background()

	_, _ = b.Write(ctx, "bucket", "key", strings.NewReader("hello"), 5)
	info, err := b.Stat(ctx, "bucket", "key")
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if info.Size != 5 {
		t.Errorf("Stat.Size = %d; want 5", info.Size)
	}
}

func TestDelete(t *testing.T) {
	dir := t.TempDir()
	b, _ := New(dir)
	ctx := context.Background()

	_, _ = b.Write(ctx, "bucket", "key", strings.NewReader("data"), 4)
	if err := b.Delete(ctx, "bucket", "key"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	ok, _ := b.Exists(ctx, "bucket", "key")
	if ok {
		t.Error("object still exists after Delete")
	}
	// Delete of non-existent should be idempotent.
	if err := b.Delete(ctx, "bucket", "key"); err != nil {
		t.Errorf("Delete (idempotent): unexpected error: %v", err)
	}
}

func TestPathTraversal(t *testing.T) {
	dir := t.TempDir()
	b, _ := New(dir)
	ctx := context.Background()

	_, err := b.Write(ctx, "bucket", "../../etc/passwd", strings.NewReader("x"), 1)
	if err == nil {
		t.Error("expected path-traversal error, got nil")
	}
}

func TestBucketDir(t *testing.T) {
	dir := t.TempDir()
	b, _ := New(dir)

	if err := b.CreateBucketDir("test-bucket"); err != nil {
		t.Fatalf("CreateBucketDir: %v", err)
	}
	// Second call should succeed (idempotent via MkdirAll).
	if err := b.CreateBucketDir("test-bucket"); err != nil {
		t.Fatalf("CreateBucketDir (2nd): %v", err)
	}
	// Delete an empty bucket.
	if err := b.DeleteBucketDir("test-bucket"); err != nil {
		t.Fatalf("DeleteBucketDir: %v", err)
	}
}

func TestReadRange(t *testing.T) {
	dir := t.TempDir()
	b, _ := New(dir)
	ctx := context.Background()

	data := "0123456789"
	_, _ = b.Write(ctx, "bucket", "key", strings.NewReader(data), int64(len(data)))

	var buf bytes.Buffer
	// ReadRange(ctx, bucket, key, w, offset, length)
	if err := b.ReadRange(ctx, "bucket", "key", &buf, 2, 4); err != nil {
		t.Fatalf("ReadRange: %v", err)
	}
	want := "2345"
	if buf.String() != want {
		t.Errorf("ReadRange = %q; want %q", buf.String(), want)
	}
}

func TestStatNotFound(t *testing.T) {
	dir := t.TempDir()
	b, _ := New(dir)
	ctx := context.Background()

	_, err := b.Stat(ctx, "bucket", "missing")
	if err == nil {
		t.Error("Stat on missing object should return error")
	}
}

func TestReadNotFound(t *testing.T) {
	dir := t.TempDir()
	b, _ := New(dir)
	ctx := context.Background()

	var buf bytes.Buffer
	err := b.Read(ctx, "bucket", "missing", &buf)
	if err == nil {
		t.Error("Read on missing object should return error")
	}
}

func TestDeleteBucketDirNonEmpty(t *testing.T) {
	dir := t.TempDir()
	b, _ := New(dir)
	ctx := context.Background()

	_ = b.CreateBucketDir("bucket")
	_, _ = b.Write(ctx, "bucket", "key", strings.NewReader("data"), 4)

	err := b.DeleteBucketDir("bucket")
	if err == nil {
		t.Error("DeleteBucketDir on non-empty bucket should return error")
	}
}

func TestModTime(t *testing.T) {
	dir := t.TempDir()
	b, _ := New(dir)
	ctx := context.Background()

	_, _ = b.Write(ctx, "bucket", "key", strings.NewReader("data"), 4)
	info, err := b.Stat(ctx, "bucket", "key")
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if info.ModTime.IsZero() {
		t.Error("ModTime should not be zero")
	}
}

func TestModTimeDirect(t *testing.T) {
	dir := t.TempDir()
	b, _ := New(dir)
	ctx := context.Background()

	_, _ = b.Write(ctx, "bucket", "mykey", strings.NewReader("data"), 4)
	mt, err := b.ModTime(ctx, "bucket", "mykey")
	if err != nil {
		t.Fatalf("ModTime: %v", err)
	}
	if mt.IsZero() {
		t.Error("ModTime should not be zero after write")
	}
}

func TestNewInvalidPath(t *testing.T) {
	// Trying to create backend with a path that is a file, not a directory.
	// Actually New just stores the root path, it doesn't create directories.
	// So let's test Write with empty bucket name instead.
	dir := t.TempDir()
	b, err := New(dir)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	ctx := context.Background()

	// Write with empty bucket should fail.
	_, err = b.Write(ctx, "", "key", strings.NewReader("x"), 1)
	if err == nil {
		t.Error("Write with empty bucket should return error")
	}

	// Write with empty key should fail.
	_, err = b.Write(ctx, "bucket", "", strings.NewReader("x"), 1)
	if err == nil {
		t.Error("Write with empty key should return error")
	}
}

func TestReadRangeNotFound(t *testing.T) {
	dir := t.TempDir()
	b, _ := New(dir)
	ctx := context.Background()

	var buf bytes.Buffer
	err := b.ReadRange(ctx, "bucket", "missing", &buf, 0, 10)
	if err == nil {
		t.Error("ReadRange on non-existent object should return error")
	}
}

func TestReadRangeFullFile(t *testing.T) {
	dir := t.TempDir()
	b, _ := New(dir)
	ctx := context.Background()

	data := "hello world"
	_, _ = b.Write(ctx, "bucket", "key", strings.NewReader(data), int64(len(data)))

	var buf bytes.Buffer
	if err := b.ReadRange(ctx, "bucket", "key", &buf, 0, int64(len(data))); err != nil {
		t.Fatalf("ReadRange full file: %v", err)
	}
	if buf.String() != data {
		t.Errorf("ReadRange full = %q; want %q", buf.String(), data)
	}
}

func TestCreateBucketDirInvalid(t *testing.T) {
	dir := t.TempDir()
	b, _ := New(dir)

	if err := b.CreateBucketDir(""); err == nil {
		t.Error("CreateBucketDir with empty name should return error")
	}
	if err := b.CreateBucketDir("bucket/with/slashes"); err == nil {
		t.Error("CreateBucketDir with slashes should return error")
	}
}

func TestDeleteBucketDirInvalid(t *testing.T) {
	dir := t.TempDir()
	b, _ := New(dir)

	if err := b.DeleteBucketDir(""); err == nil {
		t.Error("DeleteBucketDir with empty name should return error")
	}
}

func TestDeleteBucketDirNotExist(t *testing.T) {
	dir := t.TempDir()
	b, _ := New(dir)

	// Deleting a bucket that was never created should succeed (idempotent).
	if err := b.DeleteBucketDir("no-such-bucket"); err != nil {
		t.Errorf("DeleteBucketDir nonexistent: %v", err)
	}
}

func TestModTimeNotFound(t *testing.T) {
	dir := t.TempDir()
	b, _ := New(dir)
	ctx := context.Background()

	_, err := b.ModTime(ctx, "bucket", "missing")
	if err == nil {
		t.Error("ModTime for missing object should return error")
	}
}

func TestDeleteInvalidBucket(t *testing.T) {
	dir := t.TempDir()
	b, _ := New(dir)
	ctx := context.Background()

	if err := b.Delete(ctx, "", "key"); err == nil {
		t.Error("Delete with empty bucket should return error")
	}
}

func TestExistsInvalidBucket(t *testing.T) {
	dir := t.TempDir()
	b, _ := New(dir)
	ctx := context.Background()

	_, err := b.Exists(ctx, "", "key")
	if err == nil {
		t.Error("Exists with empty bucket should return error")
	}
}

func TestStatInvalidBucket(t *testing.T) {
	dir := t.TempDir()
	b, _ := New(dir)
	ctx := context.Background()

	_, err := b.Stat(ctx, "", "key")
	if err == nil {
		t.Error("Stat with empty bucket should return error")
	}
}

func TestReadInvalidBucket(t *testing.T) {
	dir := t.TempDir()
	b, _ := New(dir)
	ctx := context.Background()

	var buf bytes.Buffer
	if err := b.Read(ctx, "", "key", &buf); err == nil {
		t.Error("Read with empty bucket should return error")
	}
}

func TestWriteKeyWithSubdir(t *testing.T) {
	dir := t.TempDir()
	b, _ := New(dir)
	ctx := context.Background()

	// Writing a key with subdirectory components should succeed.
	data := "subdir data"
	_, err := b.Write(ctx, "bucket", "a/b/c/key.txt", strings.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("Write with subdir key: %v", err)
	}

	var buf bytes.Buffer
	if err := b.Read(ctx, "bucket", "a/b/c/key.txt", &buf); err != nil {
		t.Fatalf("Read subdir key: %v", err)
	}
	if buf.String() != data {
		t.Errorf("Read = %q; want %q", buf.String(), data)
	}
}
