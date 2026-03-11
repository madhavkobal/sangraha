package storage

import (
	"context"
	"fmt"

	"github.com/madhavkobal/sangraha/internal/metadata"
)

// SetBucketWebsite stores the static website configuration for a bucket.
// Pass nil to remove website hosting.
func (e *Engine) SetBucketWebsite(ctx context.Context, bucket string, cfg *metadata.WebsiteConfig) error {
	rec, err := e.meta.GetBucket(ctx, bucket)
	if err != nil {
		if isNotFound(err) {
			return &BucketNotFoundError{Name: bucket}
		}
		return fmt.Errorf("set bucket website: %w", err)
	}
	rec.Website = cfg
	if err = e.meta.PutBucket(ctx, rec); err != nil {
		return fmt.Errorf("set bucket website: store: %w", err)
	}
	return nil
}

// GetBucketWebsite returns the website configuration for the named bucket.
// Returns nil, nil if website hosting is not configured.
func (e *Engine) GetBucketWebsite(ctx context.Context, bucket string) (*metadata.WebsiteConfig, error) {
	rec, err := e.meta.GetBucket(ctx, bucket)
	if err != nil {
		if isNotFound(err) {
			return nil, &BucketNotFoundError{Name: bucket}
		}
		return nil, fmt.Errorf("get bucket website: %w", err)
	}
	return rec.Website, nil
}
