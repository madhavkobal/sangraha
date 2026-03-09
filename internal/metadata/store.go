// Package metadata defines the MetadataStore interface used by the storage
// engine to persist bucket and object records.
package metadata

import (
	"context"
	"time"
)

// BucketQuota constrains how much a bucket can hold.
type BucketQuota struct {
	MaxSizeBytes int64 `json:"max_size_bytes,omitempty"` // 0 = unlimited
	MaxObjects   int64 `json:"max_objects,omitempty"`    // 0 = unlimited
}

// WebsiteConfig holds the static website hosting configuration for a bucket.
type WebsiteConfig struct {
	IndexDocument string               `json:"index_document,omitempty"` // e.g. "index.html"
	ErrorDocument string               `json:"error_document,omitempty"` // e.g. "error.html"
	RoutingRules  []WebsiteRoutingRule `json:"routing_rules,omitempty"`
}

// WebsiteRoutingRule redirects requests matching a condition.
type WebsiteRoutingRule struct {
	Condition WebsiteCondition `json:"condition"`
	Redirect  WebsiteRedirect  `json:"redirect"`
}

// WebsiteCondition matches inbound website requests.
type WebsiteCondition struct {
	KeyPrefixEquals             string `json:"key_prefix_equals,omitempty"`
	HTTPErrorCodeReturnedEquals string `json:"http_error_code_returned_equals,omitempty"`
}

// WebsiteRedirect describes the redirect target.
type WebsiteRedirect struct {
	HostName             string `json:"host_name,omitempty"`
	Protocol             string `json:"protocol,omitempty"` // "http" or "https"
	ReplaceKeyPrefixWith string `json:"replace_key_prefix_with,omitempty"`
	ReplaceKeyWith       string `json:"replace_key_with,omitempty"`
	HTTPRedirectCode     string `json:"http_redirect_code,omitempty"` // e.g. "301"
}

// NotificationConfig holds the S3-compatible bucket notification configuration.
type NotificationConfig struct {
	// QueueConfigurations and TopicConfigurations are stored for S3 compatibility
	// but are not processed natively; use WebhookTargets for real delivery.
	QueueConfigurations []NotificationTarget `json:"queue_configurations,omitempty"`
	TopicConfigurations []NotificationTarget `json:"topic_configurations,omitempty"`
	WebhookTargets      []WebhookTarget      `json:"webhook_targets,omitempty"`
}

// NotificationTarget is an S3-compatible notification target (SQS/SNS).
type NotificationTarget struct {
	ID     string              `json:"id"`
	Arn    string              `json:"arn"`
	Events []string            `json:"events"`
	Filter *NotificationFilter `json:"filter,omitempty"`
}

// NotificationFilter restricts which object keys trigger notifications.
type NotificationFilter struct {
	KeyPrefixEquals string `json:"key_prefix_equals,omitempty"`
	KeySuffixEquals string `json:"key_suffix_equals,omitempty"`
}

// WebhookTarget is a sangraha-native webhook notification destination.
type WebhookTarget struct {
	ID     string              `json:"id"`
	URL    string              `json:"url"`
	Events []string            `json:"events"`
	Filter *NotificationFilter `json:"filter,omitempty"`
	// Secret is sent as X-Sangraha-Signature HMAC-SHA256 for payload verification.
	Secret string `json:"secret,omitempty"` //nolint:gosec // field name — not a hardcoded credential
}

// ReplicationConfig describes async object replication for a bucket.
type ReplicationConfig struct {
	Role  string            `json:"role,omitempty"` // ARN-style role (S3 compat; unused internally)
	Rules []ReplicationRule `json:"rules,omitempty"`
}

// ReplicationRule describes one replication rule.
type ReplicationRule struct {
	ID                      string          `json:"id"`
	Status                  string          `json:"status"` // "Enabled" | "Disabled"
	Filter                  LifecycleFilter `json:"filter"`
	Destination             ReplicationDest `json:"destination"`
	DeleteMarkerReplication string          `json:"delete_marker_replication,omitempty"` // "Enabled" | "Disabled"
}

// ReplicationDest is the replication destination.
type ReplicationDest struct {
	// BucketARN is the destination bucket in ARN format (arn:aws:s3:::bucket-name)
	// or sangraha URI (sangraha://host:port/bucket-name).
	BucketARN    string `json:"bucket_arn"`
	StorageClass string `json:"storage_class,omitempty"`
	Endpoint     string `json:"endpoint,omitempty"` // base URL of destination sangraha
	AccessKey    string `json:"access_key,omitempty"`
	SecretKey    string `json:"secret_key,omitempty"` //nolint:gosec // not a hardcoded credential
}

// BucketRecord holds the persisted metadata for a bucket.
type BucketRecord struct {
	Name           string            `json:"name"`
	CreatedAt      time.Time         `json:"created_at"`
	Owner          string            `json:"owner"`
	Region         string            `json:"region"`
	Versioning     string            `json:"versioning"` // "disabled" | "enabled" | "suspended"
	ACL            string            `json:"acl"`
	ObjectCount    int64             `json:"object_count"`
	TotalBytes     int64             `json:"total_bytes"`
	Policy         string            `json:"policy,omitempty"`          // JSON bucket policy
	CORSRules      []CORSRule        `json:"cors_rules,omitempty"`      // per-bucket CORS
	LifecycleRules []LifecycleRule   `json:"lifecycle_rules,omitempty"` // expiration rules
	Tags           map[string]string `json:"tags,omitempty"`            // bucket tags
	SSEAlgorithm   string            `json:"sse_algorithm,omitempty"`   // "AES256" or ""
	// Phase 3 fields.
	Quota         *BucketQuota        `json:"quota,omitempty"`         // storage quotas
	Website       *WebsiteConfig      `json:"website,omitempty"`       // static website hosting
	Notifications *NotificationConfig `json:"notifications,omitempty"` // event notifications
	Replication   *ReplicationConfig  `json:"replication,omitempty"`   // object replication
}

// CORSRule describes an S3-compatible CORS rule.
type CORSRule struct {
	ID             string   `json:"id,omitempty"`
	AllowedOrigins []string `json:"allowed_origins"`
	AllowedMethods []string `json:"allowed_methods"`
	AllowedHeaders []string `json:"allowed_headers,omitempty"`
	ExposeHeaders  []string `json:"expose_headers,omitempty"`
	MaxAgeSeconds  int      `json:"max_age_seconds,omitempty"`
}

// LifecycleRule describes an S3-compatible lifecycle rule.
type LifecycleRule struct {
	ID                              string          `json:"id"`
	Status                          string          `json:"status"` // "Enabled" | "Disabled"
	Filter                          LifecycleFilter `json:"filter"`
	ExpirationDays                  int             `json:"expiration_days,omitempty"`
	ExpirationDate                  *time.Time      `json:"expiration_date,omitempty"`
	NoncurrentVersionExpirationDays int             `json:"noncurrent_version_expiration_days,omitempty"`
	AbortIncompleteMultipartDays    int             `json:"abort_incomplete_multipart_days,omitempty"`
}

// LifecycleFilter selects objects for a lifecycle rule.
type LifecycleFilter struct {
	Prefix string            `json:"prefix,omitempty"`
	Tags   map[string]string `json:"tags,omitempty"`
}

// ObjectRecord holds the persisted metadata for a stored object.
type ObjectRecord struct {
	Bucket          string            `json:"bucket"`
	Key             string            `json:"key"`
	VersionID       string            `json:"version_id,omitempty"`
	IsDeleteMarker  bool              `json:"is_delete_marker,omitempty"`
	IsLatest        bool              `json:"is_latest,omitempty"`
	Size            int64             `json:"size"`
	ETag            string            `json:"etag"`
	ContentType     string            `json:"content_type"`
	LastModified    time.Time         `json:"last_modified"`
	Owner           string            `json:"owner"`
	UserMeta        map[string]string `json:"user_meta,omitempty"`
	StorageClass    string            `json:"storage_class"`
	Tags            map[string]string `json:"tags,omitempty"`
	SSEAlgorithm    string            `json:"sse_algorithm,omitempty"`     // "AES256"
	SSEEncryptedKey []byte            `json:"sse_encrypted_key,omitempty"` // per-object key, AES-GCM encrypted
}

// VersionRecord tracks a specific version of an object.
type VersionRecord struct {
	Bucket         string    `json:"bucket"`
	Key            string    `json:"key"`
	VersionID      string    `json:"version_id"`
	IsDeleteMarker bool      `json:"is_delete_marker"`
	IsLatest       bool      `json:"is_latest"`
	ETag           string    `json:"etag,omitempty"`
	Size           int64     `json:"size,omitempty"`
	LastModified   time.Time `json:"last_modified"`
	Owner          string    `json:"owner"`
	StorageClass   string    `json:"storage_class,omitempty"`
}

// MultipartRecord tracks an in-progress multipart upload.
type MultipartRecord struct {
	UploadID    string            `json:"upload_id"`
	Bucket      string            `json:"bucket"`
	Key         string            `json:"key"`
	Owner       string            `json:"owner"`
	ContentType string            `json:"content_type"`
	UserMeta    map[string]string `json:"user_meta,omitempty"`
	Initiated   time.Time         `json:"initiated"`
}

// PartRecord tracks a single uploaded part.
type PartRecord struct {
	UploadID     string    `json:"upload_id"`
	PartNumber   int       `json:"part_number"`
	ETag         string    `json:"etag"`
	Size         int64     `json:"size"`
	LastModified time.Time `json:"last_modified"`
}

// AccessKeyRecord holds credentials for an access key.
type AccessKeyRecord struct {
	AccessKey  string    `json:"access_key"`  //nolint:gosec // G101: field name matches pattern but is not a hardcoded credential
	SecretHash string    `json:"secret_hash"` // bcrypt hash
	SigningKey string    `json:"signing_key"` // plaintext for SigV4 (Phase 3: encrypt at rest)
	Owner      string    `json:"owner"`
	CreatedAt  time.Time `json:"created_at"`
	IsRoot     bool      `json:"is_root"`
}

// Store is the metadata persistence interface.
type Store interface {
	// --- Bucket operations ---

	// PutBucket creates or overwrites a bucket record.
	PutBucket(ctx context.Context, b BucketRecord) error
	// GetBucket returns the bucket record for name.
	GetBucket(ctx context.Context, name string) (BucketRecord, error)
	// DeleteBucket removes the bucket record.
	DeleteBucket(ctx context.Context, name string) error
	// ListBuckets returns all bucket records ordered by name.
	ListBuckets(ctx context.Context) ([]BucketRecord, error)
	// BucketExists reports whether the named bucket record exists.
	BucketExists(ctx context.Context, name string) (bool, error)

	// --- Object operations ---

	// PutObject creates or overwrites an object record.
	PutObject(ctx context.Context, o ObjectRecord) error
	// GetObject returns the object record for bucket/key.
	GetObject(ctx context.Context, bucket, key string) (ObjectRecord, error)
	// DeleteObject removes the object record.
	DeleteObject(ctx context.Context, bucket, key string) error
	// ListObjects returns object records filtered by the given options.
	ListObjects(ctx context.Context, bucket string, opts ListOptions) ([]ObjectRecord, []string, error)
	// ObjectExists reports whether the object record exists.
	ObjectExists(ctx context.Context, bucket, key string) (bool, error)
	// UpdateBucketStats atomically adjusts the object count and byte total
	// for a bucket. Pass negative values to decrement.
	UpdateBucketStats(ctx context.Context, bucket string, deltaCount, deltaBytes int64) error

	// --- Versioning operations ---

	// PutVersion stores a version record for a versioned object.
	PutVersion(ctx context.Context, v VersionRecord) error
	// GetVersion returns a specific version record.
	GetVersion(ctx context.Context, bucket, key, versionID string) (VersionRecord, error)
	// ListVersions returns all versions of bucket/key, newest first.
	ListVersions(ctx context.Context, bucket, key string) ([]VersionRecord, error)
	// ListBucketVersions returns all versions in a bucket for ListObjectVersions.
	ListBucketVersions(ctx context.Context, bucket string, opts ListOptions) ([]VersionRecord, error)
	// DeleteVersion removes a specific version record.
	DeleteVersion(ctx context.Context, bucket, key, versionID string) error
	// MarkVersionsNotLatest marks all existing versions of bucket/key as not latest.
	MarkVersionsNotLatest(ctx context.Context, bucket, key string) error

	// --- Multipart operations ---

	// PutMultipart creates an in-progress upload record.
	PutMultipart(ctx context.Context, m MultipartRecord) error
	// GetMultipart returns the multipart record for uploadID.
	GetMultipart(ctx context.Context, uploadID string) (MultipartRecord, error)
	// DeleteMultipart removes the multipart record.
	DeleteMultipart(ctx context.Context, uploadID string) error
	// ListMultiparts returns all in-progress uploads for a bucket.
	ListMultiparts(ctx context.Context, bucket string) ([]MultipartRecord, error)

	// PutPart records an uploaded part.
	PutPart(ctx context.Context, p PartRecord) error
	// ListParts returns all parts for the given upload, ordered by part number.
	ListParts(ctx context.Context, uploadID string) ([]PartRecord, error)
	// DeleteParts removes all part records for the given upload.
	DeleteParts(ctx context.Context, uploadID string) error

	// --- Access key operations ---

	// PutAccessKey stores an access key record.
	PutAccessKey(ctx context.Context, k AccessKeyRecord) error
	// GetAccessKey returns the record for the given access key.
	GetAccessKey(ctx context.Context, accessKey string) (AccessKeyRecord, error)
	// DeleteAccessKey removes the access key record.
	DeleteAccessKey(ctx context.Context, accessKey string) error
	// ListAccessKeys returns all access key records.
	ListAccessKeys(ctx context.Context) ([]AccessKeyRecord, error)

	// --- Lifecycle ---

	// Close flushes and closes the underlying store.
	Close() error
}

// ListOptions controls object listing behaviour.
type ListOptions struct {
	Prefix            string
	Delimiter         string
	ContinuationToken string
	StartAfter        string
	MaxKeys           int
}

// ErrNotFound is returned when a record does not exist.
type ErrNotFound struct {
	Kind string // "bucket", "object", "multipart", "access_key", "version"
	Name string
}

// Error implements the error interface.
func (e *ErrNotFound) Error() string {
	return "metadata: " + e.Kind + " not found: " + e.Name
}
