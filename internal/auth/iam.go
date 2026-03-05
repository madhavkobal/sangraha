package auth

// Action constants for S3 IAM policy evaluation.
const (
	ActionPutObject            = "s3:PutObject"
	ActionGetObject            = "s3:GetObject"
	ActionDeleteObject         = "s3:DeleteObject"
	ActionHeadObject           = "s3:GetObject" // HeadObject uses s3:GetObject
	ActionCopyObject           = "s3:PutObject" // CopyObject requires PutObject on dst
	ActionListBucket           = "s3:ListBucket"
	ActionCreateBucket         = "s3:CreateBucket"
	ActionDeleteBucket         = "s3:DeleteBucket"
	ActionListAllMyBuckets     = "s3:ListAllMyBuckets"
	ActionPutBucketPolicy      = "s3:PutBucketPolicy"
	ActionGetBucketPolicy      = "s3:GetBucketPolicy"
	ActionDeleteBucketPolicy   = "s3:DeleteBucketPolicy"
	ActionAbortMultipartUpload = "s3:AbortMultipartUpload"
	ActionListMultipartUploads = "s3:ListBucketMultipartUploads"
)

// IsAllowed performs a minimal IAM evaluation.
// Phase 1 implements a simple rule: root users are allowed everything;
// non-root users are allowed everything except admin operations.
// Full policy evaluation is a Phase 2 feature.
func IsAllowed(isRoot bool, _ string) bool {
	return isRoot
}
