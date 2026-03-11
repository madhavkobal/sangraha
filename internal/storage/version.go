package storage

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/google/uuid"

	"github.com/madhavkobal/sangraha/internal/metadata"
)

// VersioningStatus constants mirror the S3 API vocabulary.
const (
	VersioningDisabled  = "disabled"
	VersioningEnabled   = "enabled"
	VersioningSuspended = "suspended"
)

// SetBucketVersioning updates the versioning state for a bucket.
func (e *Engine) SetBucketVersioning(ctx context.Context, bucket, status string) error {
	rec, err := e.meta.GetBucket(ctx, bucket)
	if err != nil {
		if isNotFound(err) {
			return &BucketNotFoundError{Name: bucket}
		}
		return fmt.Errorf("set versioning: %w", err)
	}
	switch status {
	case VersioningEnabled, VersioningSuspended, VersioningDisabled:
	default:
		return fmt.Errorf("set versioning: invalid status %q", status)
	}
	rec.Versioning = status
	return e.meta.PutBucket(ctx, rec)
}

// GetBucketVersioning returns the versioning state for a bucket.
func (e *Engine) GetBucketVersioning(ctx context.Context, bucket string) (string, error) {
	rec, err := e.meta.GetBucket(ctx, bucket)
	if err != nil {
		if isNotFound(err) {
			return "", &BucketNotFoundError{Name: bucket}
		}
		return "", fmt.Errorf("get versioning: %w", err)
	}
	return rec.Versioning, nil
}

// ListObjectVersions returns all versions of all objects in a bucket.
func (e *Engine) ListObjectVersions(ctx context.Context, bucket string, opts metadata.ListOptions) ([]metadata.VersionRecord, error) {
	if _, err := e.meta.GetBucket(ctx, bucket); err != nil {
		if isNotFound(err) {
			return nil, &BucketNotFoundError{Name: bucket}
		}
		return nil, fmt.Errorf("list versions: %w", err)
	}
	return e.meta.ListBucketVersions(ctx, bucket, opts)
}

// GetObjectVersion retrieves a specific object version.
func (e *Engine) GetObjectVersion(ctx context.Context, bucket, key, versionID string) (GetObjectOutput, error) {
	ver, err := e.meta.GetVersion(ctx, bucket, key, versionID)
	if err != nil {
		if isNotFound(err) {
			return GetObjectOutput{}, &ObjectNotFoundError{Bucket: bucket, Key: key}
		}
		return GetObjectOutput{}, fmt.Errorf("get object version: %w", err)
	}
	if ver.IsDeleteMarker {
		return GetObjectOutput{}, &ObjectNotFoundError{Bucket: bucket, Key: key}
	}

	rec := metadata.ObjectRecord{
		Bucket:       ver.Bucket,
		Key:          ver.Key,
		VersionID:    ver.VersionID,
		Size:         ver.Size,
		ETag:         ver.ETag,
		LastModified: ver.LastModified,
		Owner:        ver.Owner,
		StorageClass: ver.StorageClass,
	}

	backendKey := versionedBackendKey(key, versionID)
	pr, pw := io.Pipe()
	go func() {
		rerr := e.backend.Read(ctx, bucket, backendKey, pw)
		pw.CloseWithError(rerr)
	}()
	return GetObjectOutput{Record: rec, Body: pr}, nil
}

// DeleteObjectVersion permanently removes a specific version.
func (e *Engine) DeleteObjectVersion(ctx context.Context, bucket, key, versionID string) error {
	ver, err := e.meta.GetVersion(ctx, bucket, key, versionID)
	if err != nil {
		if isNotFound(err) {
			return nil // idempotent
		}
		return fmt.Errorf("delete version: get: %w", err)
	}

	backendKey := versionedBackendKey(key, versionID)
	if !ver.IsDeleteMarker {
		if derr := e.backend.Delete(ctx, bucket, backendKey); derr != nil && !isBackendNotFound(derr) {
			return fmt.Errorf("delete version: backend: %w", derr)
		}
		_ = e.meta.UpdateBucketStats(ctx, bucket, -1, -ver.Size)
	}
	return e.meta.DeleteVersion(ctx, bucket, key, versionID)
}

// newVersionID generates a UUID-based version identifier.
func newVersionID() string {
	return uuid.NewString()
}

// versionedBackendKey returns the storage key for a versioned object copy.
// ".v." is filesystem-safe (no null bytes); version IDs are UUIDs, making
// accidental collision with real object keys negligibly unlikely.
func versionedBackendKey(key, versionID string) string {
	return key + ".v." + versionID
}

// putVersionRecord stores a version record after a successful PutObject.
func (e *Engine) putVersionRecord(ctx context.Context, rec metadata.ObjectRecord, versionID string, isLatest bool) error {
	vr := metadata.VersionRecord{
		Bucket:         rec.Bucket,
		Key:            rec.Key,
		VersionID:      versionID,
		IsDeleteMarker: false,
		IsLatest:       isLatest,
		ETag:           rec.ETag,
		Size:           rec.Size,
		LastModified:   rec.LastModified,
		Owner:          rec.Owner,
		StorageClass:   rec.StorageClass,
	}
	return e.meta.PutVersion(ctx, vr)
}

// putDeleteMarker stores a delete marker version for a versioned bucket.
func (e *Engine) putDeleteMarker(ctx context.Context, bucket, key, owner string) (string, error) {
	if err := e.meta.MarkVersionsNotLatest(ctx, bucket, key); err != nil {
		return "", fmt.Errorf("delete marker: mark not latest: %w", err)
	}
	versionID := newVersionID()
	dm := metadata.VersionRecord{
		Bucket:         bucket,
		Key:            key,
		VersionID:      versionID,
		IsDeleteMarker: true,
		IsLatest:       true,
		LastModified:   time.Now().UTC(),
		Owner:          owner,
	}
	return versionID, e.meta.PutVersion(ctx, dm)
}
