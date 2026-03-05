package auth

// CannedACL constants match the S3 canned ACL names.
const (
	ACLPrivate                = "private"
	ACLPublicRead             = "public-read"
	ACLPublicReadWrite        = "public-read-write"
	ACLAuthenticatedRead      = "authenticated-read"
	ACLBucketOwnerRead        = "bucket-owner-read"
	ACLBucketOwnerFullControl = "bucket-owner-full-control"
)

// ACLPermission describes an access level.
type ACLPermission string

const (
	// PermRead allows s3:GetObject on objects and s3:ListBucket on buckets.
	PermRead ACLPermission = "READ"
	// PermWrite allows s3:PutObject / s3:DeleteObject on a bucket.
	PermWrite ACLPermission = "WRITE"
	// PermFullControl grants all permissions.
	PermFullControl ACLPermission = "FULL_CONTROL"
)

// CheckACL returns true when the given acl permits the requested permission
// for the given subject (owner of the request).
//
// Rules (matching AWS behaviour):
//   - "private"              → only owner has FULL_CONTROL
//   - "public-read"          → anyone has READ; owner has FULL_CONTROL
//   - "public-read-write"    → anyone has READ+WRITE; owner has FULL_CONTROL
//   - "authenticated-read"   → any authenticated user has READ; owner has FULL_CONTROL
func CheckACL(acl, bucketOwner, requestOwner string, perm ACLPermission, isAuthenticated bool) bool {
	isOwner := requestOwner != "" && requestOwner == bucketOwner

	switch acl {
	case ACLPublicReadWrite:
		// Anyone (even unauthenticated) may read or write.
		return true
	case ACLPublicRead:
		// READ is open; WRITE/FULL_CONTROL require ownership.
		if perm == PermRead {
			return true
		}
		return isOwner
	case ACLAuthenticatedRead:
		// READ for authenticated users; WRITE/FULL_CONTROL require ownership.
		if perm == PermRead && isAuthenticated {
			return true
		}
		return isOwner
	default: // "private" and anything unknown
		return isOwner
	}
}
