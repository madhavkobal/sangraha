package storage

import (
	"context"
	"fmt"

	"github.com/madhavkobal/sangraha/internal/metadata"
)

// SetBucketNotification stores the notification configuration for a bucket.
// Pass nil to remove all notifications.
func (e *Engine) SetBucketNotification(ctx context.Context, bucket string, cfg *metadata.NotificationConfig) error {
	rec, err := e.meta.GetBucket(ctx, bucket)
	if err != nil {
		if isNotFound(err) {
			return &BucketNotFoundError{Name: bucket}
		}
		return fmt.Errorf("set bucket notification: %w", err)
	}
	rec.Notifications = cfg
	if err = e.meta.PutBucket(ctx, rec); err != nil {
		return fmt.Errorf("set bucket notification: store: %w", err)
	}
	return nil
}

// GetBucketNotification returns the notification configuration for the named bucket.
// Returns nil, nil if no notifications are configured.
func (e *Engine) GetBucketNotification(ctx context.Context, bucket string) (*metadata.NotificationConfig, error) {
	rec, err := e.meta.GetBucket(ctx, bucket)
	if err != nil {
		if isNotFound(err) {
			return nil, &BucketNotFoundError{Name: bucket}
		}
		return nil, fmt.Errorf("get bucket notification: %w", err)
	}
	return rec.Notifications, nil
}
