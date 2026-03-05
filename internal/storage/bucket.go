package storage

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/madhavkobal/sangraha/internal/metadata"
)

// bucketNameRe validates S3 bucket naming rules.
var bucketNameRe = regexp.MustCompile(`^[a-z0-9][a-z0-9\-]{1,61}[a-z0-9]$`)

// BucketAlreadyExistsError is returned when CreateBucket is called for a
// bucket that already exists.
type BucketAlreadyExistsError struct{ Name string }

// Error implements the error interface.
func (e *BucketAlreadyExistsError) Error() string {
	return "storage: bucket already exists: " + e.Name
}

// BucketNotFoundError is returned when an operation targets a bucket that
// does not exist.
type BucketNotFoundError struct{ Name string }

// Error implements the error interface.
func (e *BucketNotFoundError) Error() string {
	return "storage: bucket not found: " + e.Name
}

// BucketNotEmptyError is returned when DeleteBucket is called on a bucket
// that still contains objects.
type BucketNotEmptyError struct{ Name string }

// Error implements the error interface.
func (e *BucketNotEmptyError) Error() string {
	return "storage: bucket not empty: " + e.Name
}

// CreateBucket creates a new bucket with the given name owned by owner.
func (e *Engine) CreateBucket(ctx context.Context, name, owner, region string) error {
	if err := validateBucketName(name); err != nil {
		return err
	}
	exists, err := e.meta.BucketExists(ctx, name)
	if err != nil {
		return fmt.Errorf("create bucket: %w", err)
	}
	if exists {
		return &BucketAlreadyExistsError{Name: name}
	}

	// Create the backend directory for the bucket.
	type bucketCreator interface {
		CreateBucketDir(bucket string) error
	}
	if bc, ok := e.backend.(bucketCreator); ok {
		if err = bc.CreateBucketDir(name); err != nil {
			return fmt.Errorf("create bucket backend dir: %w", err)
		}
	}

	rec := metadata.BucketRecord{
		Name:       name,
		CreatedAt:  time.Now().UTC(),
		Owner:      owner,
		Region:     region,
		Versioning: "disabled",
		ACL:        "private",
	}
	if err = e.meta.PutBucket(ctx, rec); err != nil {
		return fmt.Errorf("create bucket: store: %w", err)
	}
	return nil
}

// HeadBucket returns the bucket record if the bucket exists.
func (e *Engine) HeadBucket(ctx context.Context, name string) (metadata.BucketRecord, error) {
	rec, err := e.meta.GetBucket(ctx, name)
	if err != nil {
		if isNotFound(err) {
			return metadata.BucketRecord{}, &BucketNotFoundError{Name: name}
		}
		return metadata.BucketRecord{}, fmt.Errorf("head bucket: %w", err)
	}
	return rec, nil
}

// ListBuckets returns all buckets.
func (e *Engine) ListBuckets(ctx context.Context) ([]metadata.BucketRecord, error) {
	return e.meta.ListBuckets(ctx)
}

// DeleteBucket removes a bucket. Fails if the bucket contains objects.
func (e *Engine) DeleteBucket(ctx context.Context, name string) error {
	rec, err := e.meta.GetBucket(ctx, name)
	if err != nil {
		if isNotFound(err) {
			return &BucketNotFoundError{Name: name}
		}
		return fmt.Errorf("delete bucket: %w", err)
	}
	if rec.ObjectCount > 0 {
		return &BucketNotEmptyError{Name: name}
	}

	type bucketDeleter interface {
		DeleteBucketDir(bucket string) error
	}
	if bd, ok := e.backend.(bucketDeleter); ok {
		if err = bd.DeleteBucketDir(name); err != nil {
			return fmt.Errorf("delete bucket backend dir: %w", err)
		}
	}
	return e.meta.DeleteBucket(ctx, name)
}

// validateBucketName enforces S3 bucket naming rules.
func validateBucketName(name string) error {
	if len(name) < 3 || len(name) > 63 {
		return fmt.Errorf("bucket name %q length must be between 3 and 63", name)
	}
	if !bucketNameRe.MatchString(name) {
		return fmt.Errorf("bucket name %q is invalid (must be lowercase alphanumeric and hyphens)", name)
	}
	if strings.Contains(name, "..") || strings.Contains(name, ".-") || strings.Contains(name, "-.") {
		return fmt.Errorf("bucket name %q contains invalid sequences", name)
	}
	return nil
}

// isNotFound returns true when err is a metadata.ErrNotFound.
func isNotFound(err error) bool {
	_, ok := err.(*metadata.ErrNotFound)
	return ok
}
