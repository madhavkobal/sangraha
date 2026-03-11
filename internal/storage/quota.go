package storage

import (
	"context"
	"fmt"

	"github.com/madhavkobal/sangraha/internal/metadata"
)

// QuotaExceededError is returned when a write would exceed a bucket quota.
type QuotaExceededError struct {
	Bucket   string
	Limit    int64
	Current  int64
	Incoming int64
	Kind     string // "size" or "count"
}

// Error implements the error interface.
func (e *QuotaExceededError) Error() string {
	return fmt.Sprintf("storage: quota exceeded for bucket %q: %s limit %d, current %d, incoming %d",
		e.Bucket, e.Kind, e.Limit, e.Current, e.Incoming)
}

// SetBucketQuota stores quota constraints for the named bucket.
// Pass nil to remove the quota.
func (e *Engine) SetBucketQuota(ctx context.Context, bucket string, q *metadata.BucketQuota) error {
	rec, err := e.meta.GetBucket(ctx, bucket)
	if err != nil {
		if isNotFound(err) {
			return &BucketNotFoundError{Name: bucket}
		}
		return fmt.Errorf("set bucket quota: %w", err)
	}
	rec.Quota = q
	if err = e.meta.PutBucket(ctx, rec); err != nil {
		return fmt.Errorf("set bucket quota: store: %w", err)
	}
	return nil
}

// GetBucketQuota returns the current quota for the named bucket.
// Returns nil, nil when no quota is set.
func (e *Engine) GetBucketQuota(ctx context.Context, bucket string) (*metadata.BucketQuota, error) {
	rec, err := e.meta.GetBucket(ctx, bucket)
	if err != nil {
		if isNotFound(err) {
			return nil, &BucketNotFoundError{Name: bucket}
		}
		return nil, fmt.Errorf("get bucket quota: %w", err)
	}
	return rec.Quota, nil
}

// checkQuota verifies that writing incomingBytes to bucket would not exceed
// any configured quota. bucketRec must be freshly fetched.
func checkQuota(bucketRec metadata.BucketRecord, incomingBytes int64) error {
	q := bucketRec.Quota
	if q == nil {
		return nil
	}
	if q.MaxSizeBytes > 0 && bucketRec.TotalBytes+incomingBytes > q.MaxSizeBytes {
		return &QuotaExceededError{
			Bucket:   bucketRec.Name,
			Limit:    q.MaxSizeBytes,
			Current:  bucketRec.TotalBytes,
			Incoming: incomingBytes,
			Kind:     "size",
		}
	}
	if q.MaxObjects > 0 && bucketRec.ObjectCount+1 > q.MaxObjects {
		return &QuotaExceededError{
			Bucket:   bucketRec.Name,
			Limit:    q.MaxObjects,
			Current:  bucketRec.ObjectCount,
			Incoming: 1,
			Kind:     "count",
		}
	}
	return nil
}
