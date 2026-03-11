package storage

import (
	"context"
	"fmt"
	"time"

	"github.com/madhavkobal/sangraha/internal/metadata"
)

// SetLifecycleRules stores lifecycle rules on a bucket.
func (e *Engine) SetLifecycleRules(ctx context.Context, bucket string, rules []metadata.LifecycleRule) error {
	rec, err := e.meta.GetBucket(ctx, bucket)
	if err != nil {
		if isNotFound(err) {
			return &BucketNotFoundError{Name: bucket}
		}
		return fmt.Errorf("set lifecycle: %w", err)
	}
	rec.LifecycleRules = rules
	return e.meta.PutBucket(ctx, rec)
}

// GetLifecycleRules returns the lifecycle rules for a bucket.
func (e *Engine) GetLifecycleRules(ctx context.Context, bucket string) ([]metadata.LifecycleRule, error) {
	rec, err := e.meta.GetBucket(ctx, bucket)
	if err != nil {
		if isNotFound(err) {
			return nil, &BucketNotFoundError{Name: bucket}
		}
		return nil, fmt.Errorf("get lifecycle: %w", err)
	}
	return rec.LifecycleRules, nil
}

// DeleteLifecycleRules removes all lifecycle rules from a bucket.
func (e *Engine) DeleteLifecycleRules(ctx context.Context, bucket string) error {
	return e.SetLifecycleRules(ctx, bucket, nil)
}

// ApplyLifecycle runs lifecycle rules against all objects in all buckets.
// It should be called periodically (e.g., once per day via a cron goroutine).
func (e *Engine) ApplyLifecycle(ctx context.Context) error {
	buckets, err := e.meta.ListBuckets(ctx)
	if err != nil {
		return fmt.Errorf("lifecycle: list buckets: %w", err)
	}
	for _, b := range buckets {
		if err := e.applyBucketLifecycle(ctx, b); err != nil {
			// Log but continue processing other buckets.
			_ = err
		}
	}
	return nil
}

func (e *Engine) applyBucketLifecycle(ctx context.Context, b metadata.BucketRecord) error {
	if len(b.LifecycleRules) == 0 {
		return nil
	}
	now := time.Now().UTC()

	for _, rule := range b.LifecycleRules {
		if rule.Status != "Enabled" {
			continue
		}
		if rule.ExpirationDays > 0 {
			if err := e.expireObjects(ctx, b.Name, rule, now); err != nil {
				_ = err // best-effort
			}
		}
		if rule.AbortIncompleteMultipartDays > 0 {
			if err := e.abortIncompleteMultiparts(ctx, b.Name, rule, now); err != nil {
				_ = err
			}
		}
	}
	return nil
}

func (e *Engine) expireObjects(ctx context.Context, bucket string, rule metadata.LifecycleRule, now time.Time) error {
	objects, _, err := e.meta.ListObjects(ctx, bucket, metadata.ListOptions{
		Prefix:  rule.Filter.Prefix,
		MaxKeys: 1000,
	})
	if err != nil {
		return fmt.Errorf("lifecycle expire: list: %w", err)
	}
	threshold := now.AddDate(0, 0, -rule.ExpirationDays)
	for _, obj := range objects {
		if obj.LastModified.Before(threshold) {
			if !objectMatchesFilter(obj, rule.Filter) {
				continue
			}
			_, _ = e.DeleteObject(ctx, DeleteObjectInput{
				Bucket: bucket,
				Key:    obj.Key,
			})
		}
	}
	return nil
}

func (e *Engine) abortIncompleteMultiparts(ctx context.Context, bucket string, rule metadata.LifecycleRule, now time.Time) error {
	uploads, err := e.meta.ListMultiparts(ctx, bucket)
	if err != nil {
		return fmt.Errorf("lifecycle abort multipart: list: %w", err)
	}
	threshold := now.AddDate(0, 0, -rule.AbortIncompleteMultipartDays)
	for _, upload := range uploads {
		if upload.Initiated.Before(threshold) {
			_ = e.AbortMultipartUpload(ctx, upload.UploadID)
		}
	}
	return nil
}

func objectMatchesFilter(obj metadata.ObjectRecord, filter metadata.LifecycleFilter) bool {
	if filter.Prefix != "" {
		// prefix check already done in ListObjects, but double-check
		if len(obj.Key) < len(filter.Prefix) {
			return false
		}
	}
	for k, v := range filter.Tags {
		if obj.Tags[k] != v {
			return false
		}
	}
	return true
}
