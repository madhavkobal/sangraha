package storage

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/madhavkobal/sangraha/internal/metadata"
)

// SetBucketPolicy stores a JSON IAM-style bucket policy. An empty string
// removes the policy.
func (e *Engine) SetBucketPolicy(ctx context.Context, bucket, policyJSON string) error {
	rec, err := e.meta.GetBucket(ctx, bucket)
	if err != nil {
		if isNotFound(err) {
			return &BucketNotFoundError{Name: bucket}
		}
		return fmt.Errorf("set bucket policy: %w", err)
	}
	if policyJSON != "" && !json.Valid([]byte(policyJSON)) {
		return fmt.Errorf("set bucket policy: invalid JSON")
	}
	rec.Policy = policyJSON
	return e.meta.PutBucket(ctx, rec)
}

// GetBucketPolicy returns the raw JSON policy for a bucket.
func (e *Engine) GetBucketPolicy(ctx context.Context, bucket string) (string, error) {
	rec, err := e.meta.GetBucket(ctx, bucket)
	if err != nil {
		if isNotFound(err) {
			return "", &BucketNotFoundError{Name: bucket}
		}
		return "", fmt.Errorf("get bucket policy: %w", err)
	}
	return rec.Policy, nil
}

// DeleteBucketPolicy removes the bucket policy.
func (e *Engine) DeleteBucketPolicy(ctx context.Context, bucket string) error {
	return e.SetBucketPolicy(ctx, bucket, "")
}

// SetBucketACL updates the canned ACL for a bucket.
func (e *Engine) SetBucketACL(ctx context.Context, bucket, acl string) error {
	validACLs := map[string]bool{
		"private": true, "public-read": true,
		"public-read-write": true, "authenticated-read": true,
	}
	if !validACLs[acl] {
		return fmt.Errorf("set bucket acl: invalid canned ACL %q", acl)
	}
	rec, err := e.meta.GetBucket(ctx, bucket)
	if err != nil {
		if isNotFound(err) {
			return &BucketNotFoundError{Name: bucket}
		}
		return fmt.Errorf("set bucket acl: %w", err)
	}
	rec.ACL = acl
	return e.meta.PutBucket(ctx, rec)
}

// SetBucketTags stores tags on a bucket.
func (e *Engine) SetBucketTags(ctx context.Context, bucket string, tags map[string]string) error {
	rec, err := e.meta.GetBucket(ctx, bucket)
	if err != nil {
		if isNotFound(err) {
			return &BucketNotFoundError{Name: bucket}
		}
		return fmt.Errorf("set bucket tags: %w", err)
	}
	rec.Tags = tags
	return e.meta.PutBucket(ctx, rec)
}

// GetBucketTags returns the tags for a bucket.
func (e *Engine) GetBucketTags(ctx context.Context, bucket string) (map[string]string, error) {
	rec, err := e.meta.GetBucket(ctx, bucket)
	if err != nil {
		if isNotFound(err) {
			return nil, &BucketNotFoundError{Name: bucket}
		}
		return nil, fmt.Errorf("get bucket tags: %w", err)
	}
	return rec.Tags, nil
}

// DeleteBucketTags removes all tags from a bucket.
func (e *Engine) DeleteBucketTags(ctx context.Context, bucket string) error {
	return e.SetBucketTags(ctx, bucket, nil)
}

// SetObjectTags stores tags on an object.
func (e *Engine) SetObjectTags(ctx context.Context, bucket, key string, tags map[string]string) error {
	rec, err := e.meta.GetObject(ctx, bucket, key)
	if err != nil {
		if isNotFound(err) {
			return &ObjectNotFoundError{Bucket: bucket, Key: key}
		}
		return fmt.Errorf("set object tags: %w", err)
	}
	rec.Tags = tags
	return e.meta.PutObject(ctx, rec)
}

// GetObjectTags returns the tags for an object.
func (e *Engine) GetObjectTags(ctx context.Context, bucket, key string) (map[string]string, error) {
	rec, err := e.meta.GetObject(ctx, bucket, key)
	if err != nil {
		if isNotFound(err) {
			return nil, &ObjectNotFoundError{Bucket: bucket, Key: key}
		}
		return nil, fmt.Errorf("get object tags: %w", err)
	}
	return rec.Tags, nil
}

// DeleteObjectTags removes all tags from an object.
func (e *Engine) DeleteObjectTags(ctx context.Context, bucket, key string) error {
	return e.SetObjectTags(ctx, bucket, key, nil)
}

// SetCORSRules stores CORS rules on a bucket.
func (e *Engine) SetCORSRules(ctx context.Context, bucket string, rules []metadata.CORSRule) error {
	rec, err := e.meta.GetBucket(ctx, bucket)
	if err != nil {
		if isNotFound(err) {
			return &BucketNotFoundError{Name: bucket}
		}
		return fmt.Errorf("set cors: %w", err)
	}
	rec.CORSRules = rules
	return e.meta.PutBucket(ctx, rec)
}

// GetCORSRules returns the CORS rules for a bucket.
func (e *Engine) GetCORSRules(ctx context.Context, bucket string) ([]metadata.CORSRule, error) {
	rec, err := e.meta.GetBucket(ctx, bucket)
	if err != nil {
		if isNotFound(err) {
			return nil, &BucketNotFoundError{Name: bucket}
		}
		return nil, fmt.Errorf("get cors: %w", err)
	}
	return rec.CORSRules, nil
}

// DeleteCORSRules removes all CORS rules from a bucket.
func (e *Engine) DeleteCORSRules(ctx context.Context, bucket string) error {
	return e.SetCORSRules(ctx, bucket, nil)
}

// SetBucketEncryption sets the default server-side encryption for a bucket.
func (e *Engine) SetBucketEncryption(ctx context.Context, bucket, algorithm string) error {
	if algorithm != "" && algorithm != "AES256" {
		return fmt.Errorf("set bucket encryption: unsupported algorithm %q", algorithm)
	}
	rec, err := e.meta.GetBucket(ctx, bucket)
	if err != nil {
		if isNotFound(err) {
			return &BucketNotFoundError{Name: bucket}
		}
		return fmt.Errorf("set bucket encryption: %w", err)
	}
	rec.SSEAlgorithm = algorithm
	return e.meta.PutBucket(ctx, rec)
}

// GetBucketEncryption returns the default SSE algorithm for a bucket.
func (e *Engine) GetBucketEncryption(ctx context.Context, bucket string) (string, error) {
	rec, err := e.meta.GetBucket(ctx, bucket)
	if err != nil {
		if isNotFound(err) {
			return "", &BucketNotFoundError{Name: bucket}
		}
		return "", fmt.Errorf("get bucket encryption: %w", err)
	}
	return rec.SSEAlgorithm, nil
}
