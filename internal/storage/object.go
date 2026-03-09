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
	Bucket       string
	Key          string
	Body         io.Reader
	Size         int64
	ContentType  string
	Owner        string
	UserMeta     map[string]string
	Tags         map[string]string
	SSEAlgorithm string // "AES256" or ""
}

// PutObjectOutput carries the result of a successful PutObject.
type PutObjectOutput struct {
	ETag         string
	VersionID    string
	LastModified time.Time
	Size         int64
	SSEAlgorithm string
}

// PutObject stores an object and its metadata. When the bucket has versioning
// enabled a new version ID is generated; otherwise the existing object is
// overwritten.
func (e *Engine) PutObject(ctx context.Context, in PutObjectInput) (PutObjectOutput, error) {
	bucketRec, err := e.meta.GetBucket(ctx, in.Bucket)
	if err != nil {
		if isNotFound(err) {
			return PutObjectOutput{}, &BucketNotFoundError{Name: in.Bucket}
		}
		return PutObjectOutput{}, fmt.Errorf("put object: check bucket: %w", err)
	}

	// Enforce storage quota before writing any data.
	if err = checkQuota(bucketRec, in.Size); err != nil {
		return PutObjectOutput{}, err
	}

	body, sseAlg, encryptedKey, err := e.applySSE(in, bucketRec.SSEAlgorithm)
	if err != nil {
		return PutObjectOutput{}, err
	}

	versionID, backendKey := e.resolveVersioning(in.Key, bucketRec.Versioning)

	hr := newHashReader(body)
	n, err := e.backend.Write(ctx, in.Bucket, backendKey, hr, in.Size)
	if err != nil {
		return PutObjectOutput{}, fmt.Errorf("put object: write: %w", err)
	}

	etag := `"` + hr.hexSum() + `"`
	ct := in.ContentType
	if ct == "" {
		ct = detectContentType(hr.firstBytes())
	}
	now := time.Now().UTC()

	deltaCount, deltaBytes := e.computeStatDelta(ctx, in.Bucket, in.Key, n, bucketRec.Versioning)

	rec := metadata.ObjectRecord{
		Bucket:          in.Bucket,
		Key:             in.Key,
		VersionID:       versionID,
		IsLatest:        bucketRec.Versioning == VersioningEnabled,
		Size:            n,
		ETag:            etag,
		ContentType:     ct,
		LastModified:    now,
		Owner:           in.Owner,
		UserMeta:        in.UserMeta,
		StorageClass:    "STANDARD",
		Tags:            in.Tags,
		SSEAlgorithm:    sseAlg,
		SSEEncryptedKey: encryptedKey,
	}

	if bucketRec.Versioning == VersioningEnabled {
		if merr := e.meta.MarkVersionsNotLatest(ctx, in.Bucket, in.Key); merr != nil {
			return PutObjectOutput{}, fmt.Errorf("put object: mark not latest: %w", merr)
		}
		if merr := e.putVersionRecord(ctx, rec, versionID, true); merr != nil {
			return PutObjectOutput{}, fmt.Errorf("put object: store version: %w", merr)
		}
	}

	if err = e.meta.PutObject(ctx, rec); err != nil {
		return PutObjectOutput{}, fmt.Errorf("put object: store metadata: %w", err)
	}
	if err = e.meta.UpdateBucketStats(ctx, in.Bucket, deltaCount, deltaBytes); err != nil {
		_ = err // non-fatal
	}
	// Enqueue replication if configured.
	if bucketRec.Replication != nil && e.replication != nil {
		e.replication.Enqueue(in.Bucket, in.Key, bucketRec.Replication.Rules)
	}
	// Fire webhook notifications if configured.
	if bucketRec.Notifications != nil && e.webhooks != nil {
		ev := buildObjectCreatedEvent(in.Bucket, in.Key, etag, in.Owner, n, EventObjectCreatedPut)
		e.webhooks.Dispatch(bucketRec.Notifications, ev)
	}
	return PutObjectOutput{ETag: etag, VersionID: versionID, LastModified: now, Size: n, SSEAlgorithm: sseAlg}, nil
}

// applySSE determines the SSE algorithm and wraps the body reader with encryption
// if SSE is active. Returns the (possibly wrapped) reader, effective algorithm,
// encrypted object key, and any error.
func (e *Engine) applySSE(in PutObjectInput, bucketSSEAlg string) (io.Reader, string, []byte, error) {
	sseAlg := in.SSEAlgorithm
	if sseAlg == "" {
		sseAlg = bucketSSEAlg
	}
	if sseAlg != "AES256" || masterKey == nil {
		return in.Body, sseAlg, nil, nil
	}
	objectKey, err := GenerateObjectKey()
	if err != nil {
		return nil, "", nil, fmt.Errorf("put object: sse generate key: %w", err)
	}
	encryptedKey, err := EncryptKey(objectKey)
	if err != nil {
		return nil, "", nil, fmt.Errorf("put object: sse encrypt key: %w", err)
	}
	pr, pw := io.Pipe()
	go func() {
		ew, werr := NewEncryptingWriter(pw, objectKey)
		if werr != nil {
			pw.CloseWithError(werr)
			return
		}
		_, werr = io.Copy(ew, in.Body)
		if werr == nil {
			werr = ew.Close()
		}
		pw.CloseWithError(werr)
	}()
	return pr, sseAlg, encryptedKey, nil
}

// resolveVersioning returns the versionID and backend storage key for the object.
func (e *Engine) resolveVersioning(key, versioning string) (versionID, backendKey string) {
	if versioning == VersioningEnabled {
		versionID = newVersionID()
		backendKey = versionedBackendKey(key, versionID)
	} else {
		backendKey = key
	}
	return versionID, backendKey
}

// computeStatDelta returns the (count delta, bytes delta) for bucket stats.
func (e *Engine) computeStatDelta(ctx context.Context, bucket, key string, n int64, versioning string) (int64, int64) {
	if versioning == VersioningEnabled {
		return 1, n
	}
	old, oerr := e.meta.GetObject(ctx, bucket, key)
	if oerr == nil {
		return 0, n - old.Size
	}
	return 1, n
}

// GetObjectInput carries the parameters for a GetObject operation.
type GetObjectInput struct {
	Bucket    string
	Key       string
	VersionID string // optional; if set, retrieves the specific version
}

// GetObjectOutput carries the result of a GetObject.
type GetObjectOutput struct {
	Record metadata.ObjectRecord
	Body   io.ReadCloser
}

// GetObject retrieves an object's metadata and returns a reader for its body.
// If versionID is set in GetObjectInput, the specified version is returned.
func (e *Engine) GetObject(ctx context.Context, in GetObjectInput) (GetObjectOutput, error) {
	if in.VersionID != "" {
		return e.GetObjectVersion(ctx, in.Bucket, in.Key, in.VersionID)
	}

	rec, err := e.meta.GetObject(ctx, in.Bucket, in.Key)
	if err != nil {
		if isNotFound(err) {
			return GetObjectOutput{}, &ObjectNotFoundError{Bucket: in.Bucket, Key: in.Key}
		}
		return GetObjectOutput{}, fmt.Errorf("get object: metadata: %w", err)
	}

	// Determine backend key (versioned objects use version-keyed path).
	backendKey := in.Key
	if rec.VersionID != "" {
		backendKey = versionedBackendKey(in.Key, rec.VersionID)
	}

	pr, pw := io.Pipe()
	go func() {
		rerr := e.backend.Read(ctx, in.Bucket, backendKey, pw)
		pw.CloseWithError(rerr)
	}()

	var body io.ReadCloser = pr

	// SSE decryption.
	if rec.SSEAlgorithm == "AES256" && len(rec.SSEEncryptedKey) > 0 && masterKey != nil {
		objectKey, kerr := DecryptKey(rec.SSEEncryptedKey)
		if kerr != nil {
			_ = pr.CloseWithError(kerr)
			return GetObjectOutput{}, fmt.Errorf("get object: sse decrypt key: %w", kerr)
		}
		dr, kerr := NewDecryptingReader(pr, objectKey)
		if kerr != nil {
			_ = pr.CloseWithError(kerr)
			return GetObjectOutput{}, fmt.Errorf("get object: sse decrypting reader: %w", kerr)
		}
		body = io.NopCloser(dr)
	}

	return GetObjectOutput{Record: rec, Body: body}, nil
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

// DeleteObjectInput carries parameters for a DeleteObject call.
type DeleteObjectInput struct {
	Bucket    string
	Key       string
	VersionID string // if set, permanently deletes that version
	Owner     string
}

// DeleteObjectOutput carries the result of a DeleteObject call.
type DeleteObjectOutput struct {
	VersionID    string // version ID of the delete marker (versioned buckets)
	DeleteMarker bool
}

// DeleteObject removes an object and its metadata. For versioned buckets a
// delete marker is created unless VersionID is specified (permanent delete).
func (e *Engine) DeleteObject(ctx context.Context, in DeleteObjectInput) (DeleteObjectOutput, error) {
	if in.VersionID != "" {
		// Permanent deletion of a specific version.
		if err := e.DeleteObjectVersion(ctx, in.Bucket, in.Key, in.VersionID); err != nil {
			return DeleteObjectOutput{}, err
		}
		return DeleteObjectOutput{VersionID: in.VersionID}, nil
	}

	bkt, err := e.meta.GetBucket(ctx, in.Bucket)
	if err != nil {
		if isNotFound(err) {
			return DeleteObjectOutput{}, nil // idempotent
		}
		return DeleteObjectOutput{}, fmt.Errorf("delete object: get bucket: %w", err)
	}

	// Versioning-enabled: create delete marker.
	if bkt.Versioning == VersioningEnabled {
		vid, merr := e.putDeleteMarker(ctx, in.Bucket, in.Key, in.Owner)
		if merr != nil {
			return DeleteObjectOutput{}, fmt.Errorf("delete object: put delete marker: %w", merr)
		}
		// Remove from the main object index so GetObject returns 404.
		if derr := e.meta.DeleteObject(ctx, in.Bucket, in.Key); derr != nil && !isNotFound(derr) {
			return DeleteObjectOutput{}, fmt.Errorf("delete object: remove index: %w", derr)
		}
		return DeleteObjectOutput{VersionID: vid, DeleteMarker: true}, nil
	}

	// Non-versioned: delete normally.
	rec, err := e.meta.GetObject(ctx, in.Bucket, in.Key)
	if err != nil {
		if isNotFound(err) {
			return DeleteObjectOutput{}, nil // idempotent
		}
		return DeleteObjectOutput{}, fmt.Errorf("delete object: metadata get: %w", err)
	}

	backendKey := in.Key
	if rec.VersionID != "" {
		backendKey = versionedBackendKey(in.Key, rec.VersionID)
	}
	if err = e.backend.Delete(ctx, in.Bucket, backendKey); err != nil {
		if !isBackendNotFound(err) {
			return DeleteObjectOutput{}, fmt.Errorf("delete object: backend: %w", err)
		}
	}
	if err = e.meta.DeleteObject(ctx, in.Bucket, in.Key); err != nil {
		return DeleteObjectOutput{}, fmt.Errorf("delete object: metadata delete: %w", err)
	}
	_ = e.meta.UpdateBucketStats(ctx, in.Bucket, -1, -rec.Size)
	// Fire webhook notifications if configured.
	if bkt.Notifications != nil && e.webhooks != nil {
		ev := buildObjectRemovedEvent(in.Bucket, in.Key, in.Owner)
		e.webhooks.Dispatch(bkt.Notifications, ev)
	}
	return DeleteObjectOutput{}, nil
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

	srcBackendKey := srcKey
	if src.VersionID != "" {
		srcBackendKey = versionedBackendKey(srcKey, src.VersionID)
	}

	pr, pw := io.Pipe()
	errCh := make(chan error, 1)
	go func() {
		rerr := e.backend.Read(ctx, srcBucket, srcBackendKey, pw)
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
func (e *Engine) DeleteObjects(ctx context.Context, bucket string, keys []string, owner string) (deleted []string, errs map[string]error) {
	errs = make(map[string]error)
	for _, key := range keys {
		if _, err := e.DeleteObject(ctx, DeleteObjectInput{Bucket: bucket, Key: key, Owner: owner}); err != nil {
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
