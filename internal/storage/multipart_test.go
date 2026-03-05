package storage

import (
	"bytes"
	"context"
	"io"
	"strings"
	"testing"
)

func TestCreateMultipartUpload(t *testing.T) {
	e := newTestEngine(t)
	ctx := context.Background()

	_ = e.CreateBucket(ctx, "bucket", "root", "")

	uploadID, err := e.CreateMultipartUpload(ctx, "bucket", "big/file.bin", "root", "application/octet-stream", nil)
	if err != nil {
		t.Fatalf("CreateMultipartUpload: %v", err)
	}
	if uploadID == "" {
		t.Error("CreateMultipartUpload should return a non-empty upload ID")
	}
}

func TestCreateMultipartUploadMissingBucket(t *testing.T) {
	e := newTestEngine(t)
	ctx := context.Background()

	_, err := e.CreateMultipartUpload(ctx, "missing-bucket", "key", "root", "", nil)
	if err == nil {
		t.Error("CreateMultipartUpload on missing bucket should return error")
	}
}

func TestUploadPart(t *testing.T) {
	e := newTestEngine(t)
	ctx := context.Background()

	_ = e.CreateBucket(ctx, "bucket", "root", "")
	uploadID, _ := e.CreateMultipartUpload(ctx, "bucket", "big.bin", "root", "", nil)

	data := strings.Repeat("A", 1024)
	etag, err := e.UploadPart(ctx, UploadPartInput{
		UploadID:   uploadID,
		PartNumber: 1,
		Body:       strings.NewReader(data),
		Size:       int64(len(data)),
	})
	if err != nil {
		t.Fatalf("UploadPart: %v", err)
	}
	if etag == "" {
		t.Error("UploadPart should return ETag")
	}
}

func TestUploadPartMissingUpload(t *testing.T) {
	e := newTestEngine(t)
	ctx := context.Background()

	_, err := e.UploadPart(ctx, UploadPartInput{
		UploadID:   "nonexistent-upload-id",
		PartNumber: 1,
		Body:       strings.NewReader("data"),
		Size:       4,
	})
	if err == nil {
		t.Error("UploadPart on missing upload should return error")
	}
}

func TestListParts(t *testing.T) {
	e := newTestEngine(t)
	ctx := context.Background()

	_ = e.CreateBucket(ctx, "bucket", "root", "")
	uploadID, _ := e.CreateMultipartUpload(ctx, "bucket", "key", "root", "", nil)

	data := strings.Repeat("B", 1024)
	for i := 1; i <= 3; i++ {
		_, _ = e.UploadPart(ctx, UploadPartInput{
			UploadID:   uploadID,
			PartNumber: i,
			Body:       strings.NewReader(data),
			Size:       int64(len(data)),
		})
	}

	parts, err := e.ListParts(ctx, uploadID)
	if err != nil {
		t.Fatalf("ListParts: %v", err)
	}
	if len(parts) != 3 {
		t.Errorf("got %d parts; want 3", len(parts))
	}
}

func TestCompleteMultipartUpload(t *testing.T) {
	e := newTestEngine(t)
	ctx := context.Background()

	_ = e.CreateBucket(ctx, "bucket", "root", "")
	uploadID, _ := e.CreateMultipartUpload(ctx, "bucket", "assembled.bin", "root", "application/octet-stream", nil)

	// Upload 2 parts.
	part1 := strings.Repeat("X", 1024)
	part2 := strings.Repeat("Y", 512)

	etag1, _ := e.UploadPart(ctx, UploadPartInput{
		UploadID: uploadID, PartNumber: 1,
		Body: strings.NewReader(part1), Size: int64(len(part1)),
	})
	etag2, _ := e.UploadPart(ctx, UploadPartInput{
		UploadID: uploadID, PartNumber: 2,
		Body: strings.NewReader(part2), Size: int64(len(part2)),
	})

	rec, err := e.CompleteMultipartUpload(ctx, CompleteMultipartInput{
		UploadID: uploadID,
		Parts: []CompletePart{
			{PartNumber: 1, ETag: etag1},
			{PartNumber: 2, ETag: etag2},
		},
	})
	if err != nil {
		t.Fatalf("CompleteMultipartUpload: %v", err)
	}
	if rec.Key != "assembled.bin" {
		t.Errorf("key = %q; want %q", rec.Key, "assembled.bin")
	}
	if rec.Size != int64(len(part1)+len(part2)) {
		t.Errorf("size = %d; want %d", rec.Size, len(part1)+len(part2))
	}

	// Verify the assembled object is readable.
	out, err := e.GetObject(ctx, GetObjectInput{Bucket: "bucket", Key: "assembled.bin"})
	if err != nil {
		t.Fatalf("GetObject after complete: %v", err)
	}
	var buf bytes.Buffer
	_, _ = io.Copy(&buf, out.Body)
	_ = out.Body.Close()
	want := part1 + part2
	if buf.String() != want {
		t.Errorf("assembled content mismatch: got len=%d, want len=%d", buf.Len(), len(want))
	}
}

func TestAbortMultipartUpload(t *testing.T) {
	e := newTestEngine(t)
	ctx := context.Background()

	_ = e.CreateBucket(ctx, "bucket", "root", "")
	uploadID, _ := e.CreateMultipartUpload(ctx, "bucket", "key", "root", "", nil)

	_, _ = e.UploadPart(ctx, UploadPartInput{
		UploadID:   uploadID,
		PartNumber: 1,
		Body:       strings.NewReader("data"),
		Size:       4,
	})

	if err := e.AbortMultipartUpload(ctx, uploadID); err != nil {
		t.Fatalf("AbortMultipartUpload: %v", err)
	}

	// Verify the upload is gone.
	_, err := e.ListParts(ctx, uploadID)
	if err == nil {
		t.Error("ListParts after abort should return error")
	}
}

func TestAbortMultipartUploadMissing(t *testing.T) {
	e := newTestEngine(t)
	ctx := context.Background()

	err := e.AbortMultipartUpload(ctx, "nonexistent-id")
	if err == nil {
		t.Error("AbortMultipartUpload on missing upload should return error")
	}
}

func TestListMultipartUploads(t *testing.T) {
	e := newTestEngine(t)
	ctx := context.Background()

	_ = e.CreateBucket(ctx, "bucket", "root", "")
	_, _ = e.CreateMultipartUpload(ctx, "bucket", "file1", "root", "", nil)
	_, _ = e.CreateMultipartUpload(ctx, "bucket", "file2", "root", "", nil)

	uploads, err := e.ListMultipartUploads(ctx, "bucket")
	if err != nil {
		t.Fatalf("ListMultipartUploads: %v", err)
	}
	if len(uploads) != 2 {
		t.Errorf("got %d uploads; want 2", len(uploads))
	}
}

func TestMultipartNotFoundError(t *testing.T) {
	e := &MultipartNotFoundError{UploadID: "missing-id"}
	if e.Error() == "" {
		t.Error("MultipartNotFoundError.Error() should not be empty")
	}
}
