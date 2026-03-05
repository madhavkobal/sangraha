package storage

import (
	"bytes"
	"context"
	"crypto/md5" //nolint:gosec // MD5 is required by the S3 spec for ETag computation
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/madhavkobal/sangraha/internal/backend"
	"github.com/madhavkobal/sangraha/internal/metadata"
)

// ObjectNotFoundError is returned when an object does not exist.
type ObjectNotFoundError struct {
	Bucket string
	Key    string
}

// Error implements the error interface.
func (e *ObjectNotFoundError) Error() string {
	return "storage: object not found: " + e.Bucket + "/" + e.Key
}

// PutObjectInput carries all the parameters for a PutObject operation.
type PutObjectInput struct {
	Bucket      string
	Key         string
	Body        io.Reader
	Size        int64
	ContentType string
	Owner       string
	UserMeta    map[string]string
}

// PutObjectOutput carries the result of a successful PutObject.
type PutObjectOutput struct {
	ETag         string
	LastModified time.Time
	Size         int64
}

// PutObject stores an object and its metadata.
func (e *Engine) PutObject(ctx context.Context, in PutObjectInput) (PutObjectOutput, error) {
	// Validate that the bucket exists.
	exists, err := e.meta.BucketExists(ctx, in.Bucket)
	if err != nil {
		return PutObjectOutput{}, fmt.Errorf("put object: check bucket: %w", err)
	}
	if !exists {
		return PutObjectOutput{}, &BucketNotFoundError{Name: in.Bucket}
	}

	// Wrap the body in a hash reader to compute the ETag while streaming.
	hr := newHashReader(in.Body)
	n, err := e.backend.Write(ctx, in.Bucket, in.Key, hr, in.Size)
	if err != nil {
		return PutObjectOutput{}, fmt.Errorf("put object: write: %w", err)
	}

	etag := `"` + hr.hexSum() + `"`
	ct := in.ContentType
	if ct == "" {
		ct = detectContentType(hr.firstBytes())
	}
	now := time.Now().UTC()

	// Check if the object already exists to compute stat delta.
	var deltaCount, deltaBytes int64
	old, err := e.meta.GetObject(ctx, in.Bucket, in.Key)
	if err == nil {
		// Overwrite: same count, adjust byte delta.
		deltaBytes = n - old.Size
	} else {
		deltaCount = 1
		deltaBytes = n
	}

	rec := metadata.ObjectRecord{
		Bucket:       in.Bucket,
		Key:          in.Key,
		Size:         n,
		ETag:         etag,
		ContentType:  ct,
		LastModified: now,
		Owner:        in.Owner,
		UserMeta:     in.UserMeta,
		StorageClass: "STANDARD",
	}
	if err = e.meta.PutObject(ctx, rec); err != nil {
		return PutObjectOutput{}, fmt.Errorf("put object: store metadata: %w", err)
	}
	if err = e.meta.UpdateBucketStats(ctx, in.Bucket, deltaCount, deltaBytes); err != nil {
		// Non-fatal: stats are best-effort.
		_ = err
	}
	return PutObjectOutput{ETag: etag, LastModified: now, Size: n}, nil
}

// GetObjectInput carries the parameters for a GetObject operation.
type GetObjectInput struct {
	Bucket string
	Key    string
}

// GetObjectOutput carries the result of a GetObject.
type GetObjectOutput struct {
	Record metadata.ObjectRecord
	Body   io.ReadCloser
}

// GetObject retrieves an object's metadata and returns a reader for its body.
func (e *Engine) GetObject(ctx context.Context, in GetObjectInput) (GetObjectOutput, error) {
	rec, err := e.meta.GetObject(ctx, in.Bucket, in.Key)
	if err != nil {
		if isNotFound(err) {
			return GetObjectOutput{}, &ObjectNotFoundError{Bucket: in.Bucket, Key: in.Key}
		}
		return GetObjectOutput{}, fmt.Errorf("get object: metadata: %w", err)
	}

	pr, pw := io.Pipe()
	go func() {
		rerr := e.backend.Read(ctx, in.Bucket, in.Key, pw)
		pw.CloseWithError(rerr)
	}()
	return GetObjectOutput{Record: rec, Body: pr}, nil
}

// HeadObject returns the object metadata without reading the body.
func (e *Engine) HeadObject(ctx context.Context, bucket, key string) (metadata.ObjectRecord, error) {
	rec, err := e.meta.GetObject(ctx, bucket, key)
	if err != nil {
		if isNotFound(err) {
			return metadata.ObjectRecord{}, &ObjectNotFoundError{Bucket: bucket, Key: key}
		}
		return metadata.ObjectRecord{}, fmt.Errorf("head object: %w", err)
	}
	return rec, nil
}

// DeleteObject removes an object and its metadata.
func (e *Engine) DeleteObject(ctx context.Context, bucket, key string) error {
	rec, err := e.meta.GetObject(ctx, bucket, key)
	if err != nil {
		if isNotFound(err) {
			return nil // idempotent
		}
		return fmt.Errorf("delete object: metadata get: %w", err)
	}

	if err = e.backend.Delete(ctx, bucket, key); err != nil {
		if !isBackendNotFound(err) {
			return fmt.Errorf("delete object: backend: %w", err)
		}
	}
	if err = e.meta.DeleteObject(ctx, bucket, key); err != nil {
		return fmt.Errorf("delete object: metadata delete: %w", err)
	}
	_ = e.meta.UpdateBucketStats(ctx, bucket, -1, -rec.Size)
	return nil
}

// CopyObject copies src to dst, potentially across buckets.
func (e *Engine) CopyObject(ctx context.Context, srcBucket, srcKey, dstBucket, dstKey, owner string) (metadata.ObjectRecord, error) {
	src, err := e.meta.GetObject(ctx, srcBucket, srcKey)
	if err != nil {
		if isNotFound(err) {
			return metadata.ObjectRecord{}, &ObjectNotFoundError{Bucket: srcBucket, Key: srcKey}
		}
		return metadata.ObjectRecord{}, fmt.Errorf("copy object: get src: %w", err)
	}
	exists, err := e.meta.BucketExists(ctx, dstBucket)
	if err != nil {
		return metadata.ObjectRecord{}, fmt.Errorf("copy object: check dst bucket: %w", err)
	}
	if !exists {
		return metadata.ObjectRecord{}, &BucketNotFoundError{Name: dstBucket}
	}

	pr, pw := io.Pipe()
	errCh := make(chan error, 1)
	go func() {
		rerr := e.backend.Read(ctx, srcBucket, srcKey, pw)
		pw.CloseWithError(rerr)
		errCh <- rerr
	}()

	n, err := e.backend.Write(ctx, dstBucket, dstKey, pr, src.Size)
	if err != nil {
		return metadata.ObjectRecord{}, fmt.Errorf("copy object: write dst: %w", err)
	}
	if rerr := <-errCh; rerr != nil {
		return metadata.ObjectRecord{}, fmt.Errorf("copy object: read src: %w", rerr)
	}
	now := time.Now().UTC()
	rec := metadata.ObjectRecord{
		Bucket:       dstBucket,
		Key:          dstKey,
		Size:         n,
		ETag:         src.ETag,
		ContentType:  src.ContentType,
		LastModified: now,
		Owner:        owner,
		UserMeta:     src.UserMeta,
		StorageClass: "STANDARD",
	}
	if err = e.meta.PutObject(ctx, rec); err != nil {
		return metadata.ObjectRecord{}, fmt.Errorf("copy object: store: %w", err)
	}
	_ = e.meta.UpdateBucketStats(ctx, dstBucket, 1, n)
	return rec, nil
}

// ListObjects wraps the metadata store list with validation.
func (e *Engine) ListObjects(ctx context.Context, bucket string, opts metadata.ListOptions) ([]metadata.ObjectRecord, []string, error) {
	if _, err := e.meta.GetBucket(ctx, bucket); err != nil {
		if isNotFound(err) {
			return nil, nil, &BucketNotFoundError{Name: bucket}
		}
		return nil, nil, fmt.Errorf("list objects: %w", err)
	}
	return e.meta.ListObjects(ctx, bucket, opts)
}

// DeleteObjects deletes multiple objects. Returns per-object results.
func (e *Engine) DeleteObjects(ctx context.Context, bucket string, keys []string) (deleted []string, errs map[string]error) {
	errs = make(map[string]error)
	for _, key := range keys {
		if err := e.DeleteObject(ctx, bucket, key); err != nil {
			errs[key] = err
		} else {
			deleted = append(deleted, key)
		}
	}
	return deleted, errs
}

// isBackendNotFound returns true when the error is a backend.ErrNotFound.
func isBackendNotFound(err error) bool {
	_, ok := err.(*backend.ErrNotFound)
	return ok
}

// ----------------------------------------------------------------------------
// hashReader wraps an io.Reader and computes the MD5 digest while streaming.
// ----------------------------------------------------------------------------

type hashReader struct {
	r io.Reader
	h interface {
		Write([]byte) (int, error)
		Sum([]byte) []byte
	}
	firstBuf  []byte
	firstDone bool
}

func newHashReader(r io.Reader) *hashReader {
	return &hashReader{r: r, h: md5.New()} //nolint:gosec // S3 ETag is MD5
}

func (hr *hashReader) Read(p []byte) (int, error) {
	n, err := hr.r.Read(p)
	if n > 0 {
		_, _ = hr.h.Write(p[:n])
		if !hr.firstDone && len(hr.firstBuf) < 512 {
			take := 512 - len(hr.firstBuf)
			if take > n {
				take = n
			}
			hr.firstBuf = append(hr.firstBuf, p[:take]...)
			if len(hr.firstBuf) >= 512 {
				hr.firstDone = true
			}
		}
	}
	return n, err
}

func (hr *hashReader) hexSum() string {
	return hex.EncodeToString(hr.h.Sum(nil))
}

func (hr *hashReader) firstBytes() []byte {
	return hr.firstBuf
}

// detectContentType sniffs the content type from the first bytes.
func detectContentType(data []byte) string {
	if len(data) == 0 {
		return "application/octet-stream"
	}
	ct := http.DetectContentType(data)
	// Normalise the generic fallback.
	if strings.HasPrefix(ct, "application/octet-stream") {
		return "application/octet-stream"
	}
	return ct
}

// newMD5 is a helper that returns an MD5 hash of data.
func newMD5(data []byte) string { //nolint:gosec // Used for ETag, not security
	h := md5.Sum(data) //nolint:gosec
	return hex.EncodeToString(h[:])
}

// compositeMD5 builds the multipart composite ETag:
//
//	"<md5-of-part-md5s>-<numParts>"
func compositeMD5(partETags []string) string {
	h := md5.New() //nolint:gosec
	for _, etag := range partETags {
		stripped := strings.Trim(etag, `"`)
		b, _ := hex.DecodeString(stripped)
		_, _ = io.Copy(h, bytes.NewReader(b))
	}
	return fmt.Sprintf(`"%s-%d"`, hex.EncodeToString(h.Sum(nil)), len(partETags))
}
