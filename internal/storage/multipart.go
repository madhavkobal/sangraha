package storage

import (
	"context"
	"fmt"
	"io"
	"sort"
	"time"

	"github.com/google/uuid"

	"github.com/madhavkobal/sangraha/internal/metadata"
)

const (
	// MinPartSize is the minimum size for all parts except the last (5 MiB).
	MinPartSize = 5 * 1024 * 1024
)

// MultipartNotFoundError is returned when an upload ID is not found.
type MultipartNotFoundError struct{ UploadID string }

// Error implements the error interface.
func (e *MultipartNotFoundError) Error() string {
	return "storage: multipart upload not found: " + e.UploadID
}

// CreateMultipartUpload initialises a new multipart upload and returns its ID.
func (e *Engine) CreateMultipartUpload(ctx context.Context, bucket, key, owner, contentType string, userMeta map[string]string) (string, error) {
	exists, err := e.meta.BucketExists(ctx, bucket)
	if err != nil {
		return "", fmt.Errorf("create multipart: check bucket: %w", err)
	}
	if !exists {
		return "", &BucketNotFoundError{Name: bucket}
	}

	uploadID := uuid.New().String()
	rec := metadata.MultipartRecord{
		UploadID:    uploadID,
		Bucket:      bucket,
		Key:         key,
		Owner:       owner,
		ContentType: contentType,
		UserMeta:    userMeta,
		Initiated:   time.Now().UTC(),
	}
	if err = e.meta.PutMultipart(ctx, rec); err != nil {
		return "", fmt.Errorf("create multipart: store: %w", err)
	}
	return uploadID, nil
}

// UploadPartInput carries the parameters for an UploadPart operation.
type UploadPartInput struct {
	UploadID   string
	PartNumber int
	Body       io.Reader
	Size       int64
}

// UploadPart stores a single part and records its ETag.
func (e *Engine) UploadPart(ctx context.Context, in UploadPartInput) (string, error) {
	if _, err := e.meta.GetMultipart(ctx, in.UploadID); err != nil {
		if isNotFound(err) {
			return "", &MultipartNotFoundError{UploadID: in.UploadID}
		}
		return "", fmt.Errorf("upload part: get upload: %w", err)
	}
	// Parts are stored under a synthetic key: .<uploadID>.<partNumber>
	partKey := fmt.Sprintf(".multipart/%s/%05d", in.UploadID, in.PartNumber)

	// Use a bucket-agnostic path; the multipart upload record knows the bucket.
	m, err := e.meta.GetMultipart(ctx, in.UploadID)
	if err != nil {
		return "", fmt.Errorf("upload part: resolve bucket: %w", err)
	}

	hr := newHashReader(in.Body)
	_, err = e.backend.Write(ctx, m.Bucket, partKey, hr, in.Size)
	if err != nil {
		return "", fmt.Errorf("upload part: write: %w", err)
	}
	etag := `"` + hr.hexSum() + `"`

	pr := metadata.PartRecord{
		UploadID:     in.UploadID,
		PartNumber:   in.PartNumber,
		ETag:         etag,
		Size:         in.Size,
		LastModified: time.Now().UTC(),
	}
	if err = e.meta.PutPart(ctx, pr); err != nil {
		return "", fmt.Errorf("upload part: store: %w", err)
	}
	return etag, nil
}

// CompleteMultipartInput carries the parameters for CompleteMultipartUpload.
type CompleteMultipartInput struct {
	UploadID string
	Parts    []CompletePart
}

// CompletePart identifies a part to include in the assembled object.
type CompletePart struct {
	PartNumber int
	ETag       string
}

// CompleteMultipartUpload assembles all parts into the final object.
func (e *Engine) CompleteMultipartUpload(ctx context.Context, in CompleteMultipartInput) (metadata.ObjectRecord, error) {
	m, err := e.meta.GetMultipart(ctx, in.UploadID)
	if err != nil {
		if isNotFound(err) {
			return metadata.ObjectRecord{}, &MultipartNotFoundError{UploadID: in.UploadID}
		}
		return metadata.ObjectRecord{}, fmt.Errorf("complete multipart: get upload: %w", err)
	}

	// Enforce quota before assembling.
	bucketRec, err := e.meta.GetBucket(ctx, m.Bucket)
	if err != nil {
		return metadata.ObjectRecord{}, fmt.Errorf("complete multipart: get bucket: %w", err)
	}
	// Load part records once; used for quota check and size accounting.
	allParts, err := e.meta.ListParts(ctx, in.UploadID)
	if err != nil {
		return metadata.ObjectRecord{}, fmt.Errorf("complete multipart: list parts: %w", err)
	}

	quotaSize := multipartTotalSize(allParts, in.Parts)
	if qerr := checkQuota(bucketRec, quotaSize); qerr != nil {
		return metadata.ObjectRecord{}, qerr
	}

	// Sort parts by part number.
	sort.Slice(in.Parts, func(i, j int) bool {
		return in.Parts[i].PartNumber < in.Parts[j].PartNumber
	})

	combined, partETags, totalSize := e.assemblePartReaders(ctx, m.Bucket, in.UploadID, in.Parts, allParts)
	_, err = e.backend.Write(ctx, m.Bucket, m.Key, combined, totalSize)
	if err != nil {
		return metadata.ObjectRecord{}, fmt.Errorf("complete multipart: write final: %w", err)
	}

	etag := compositeMD5(partETags)
	now := time.Now().UTC()
	rec := metadata.ObjectRecord{
		Bucket:       m.Bucket,
		Key:          m.Key,
		Size:         totalSize,
		ETag:         etag,
		ContentType:  m.ContentType,
		LastModified: now,
		Owner:        m.Owner,
		UserMeta:     m.UserMeta,
		StorageClass: "STANDARD",
	}
	if err = e.meta.PutObject(ctx, rec); err != nil {
		return metadata.ObjectRecord{}, fmt.Errorf("complete multipart: store object: %w", err)
	}
	_ = e.meta.UpdateBucketStats(ctx, m.Bucket, 1, totalSize)
	e.fireObjectCreatedSideEffects(m.Bucket, m.Key, rec.ETag, m.Owner, totalSize, bucketRec,
		EventObjectCreatedMultipartCompleted)

	// Clean up the in-progress upload record and part data.
	e.cleanupMultipart(ctx, in.UploadID, m.Bucket, len(in.Parts))

	return rec, nil
}

// AbortMultipartUpload cancels an in-progress upload and cleans up parts.
func (e *Engine) AbortMultipartUpload(ctx context.Context, uploadID string) error {
	m, err := e.meta.GetMultipart(ctx, uploadID)
	if err != nil {
		if isNotFound(err) {
			return &MultipartNotFoundError{UploadID: uploadID}
		}
		return fmt.Errorf("abort multipart: get upload: %w", err)
	}
	parts, err := e.meta.ListParts(ctx, uploadID)
	if err != nil {
		return fmt.Errorf("abort multipart: list parts: %w", err)
	}
	for _, p := range parts {
		partKey := fmt.Sprintf(".multipart/%s/%05d", uploadID, p.PartNumber)
		_ = e.backend.Delete(ctx, m.Bucket, partKey)
	}
	e.cleanupMultipart(ctx, uploadID, m.Bucket, len(parts))
	return nil
}

// ListParts returns all recorded parts for an upload.
func (e *Engine) ListParts(ctx context.Context, uploadID string) ([]metadata.PartRecord, error) {
	if _, err := e.meta.GetMultipart(ctx, uploadID); err != nil {
		if isNotFound(err) {
			return nil, &MultipartNotFoundError{UploadID: uploadID}
		}
		return nil, fmt.Errorf("list parts: %w", err)
	}
	return e.meta.ListParts(ctx, uploadID)
}

// ListMultipartUploads returns all in-progress uploads for a bucket.
func (e *Engine) ListMultipartUploads(ctx context.Context, bucket string) ([]metadata.MultipartRecord, error) {
	return e.meta.ListMultiparts(ctx, bucket)
}

// multipartTotalSize returns the sum of the sizes of the selected parts.
func multipartTotalSize(allParts []metadata.PartRecord, selected []CompletePart) int64 {
	sizeByNumber := make(map[int]int64, len(allParts))
	for _, p := range allParts {
		sizeByNumber[p.PartNumber] = p.Size
	}
	var total int64
	for _, cp := range selected {
		total += sizeByNumber[cp.PartNumber]
	}
	return total
}

// assemblePartReaders creates a concatenated reader over all selected parts' backend data,
// and returns it along with the per-part ETags and total assembled size.
func (e *Engine) assemblePartReaders(ctx context.Context, bucket, uploadID string, selected []CompletePart, allParts []metadata.PartRecord) (io.Reader, []string, int64) {
	sizeByNumber := make(map[int]int64, len(allParts))
	for _, p := range allParts {
		sizeByNumber[p.PartNumber] = p.Size
	}
	readers := make([]io.Reader, 0, len(selected))
	etags := make([]string, 0, len(selected))
	var totalSize int64
	for _, cp := range selected {
		partKey := fmt.Sprintf(".multipart/%s/%05d", uploadID, cp.PartNumber)
		pr, pw := io.Pipe()
		go func(key string, pw *io.PipeWriter) {
			rerr := e.backend.Read(ctx, bucket, key, pw)
			pw.CloseWithError(rerr)
		}(partKey, pw)
		readers = append(readers, pr)
		totalSize += sizeByNumber[cp.PartNumber]
		etags = append(etags, cp.ETag)
	}
	return io.MultiReader(readers...), etags, totalSize
}

func (e *Engine) cleanupMultipart(ctx context.Context, uploadID, bucket string, numParts int) {
	_ = e.meta.DeleteMultipart(ctx, uploadID)
	_ = e.meta.DeleteParts(ctx, uploadID)
	// Remove the part storage directories (best-effort).
	for i := 1; i <= numParts; i++ {
		partKey := fmt.Sprintf(".multipart/%s/%05d", uploadID, i)
		_ = e.backend.Delete(ctx, bucket, partKey)
	}
}
