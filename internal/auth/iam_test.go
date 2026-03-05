package auth

import "testing"

func TestIsAllowed(t *testing.T) {
	// Root users are allowed everything.
	if !IsAllowed(true, "s3:PutObject", "alice", "mybucket", "", "") {
		t.Error("IsAllowed(root) should return true")
	}
	if !IsAllowed(true, "s3:DeleteBucket", "alice", "mybucket", "", "") {
		t.Error("IsAllowed(root) should return true for any action")
	}
	// Non-root users with no policy are allowed (Phase 2 default: allow all authenticated).
	if !IsAllowed(false, "s3:PutObject", "alice", "mybucket", "", "") {
		t.Error("IsAllowed(non-root, no policy) should return true")
	}
	// Non-root users with an explicit Deny policy are denied.
	denyPolicy := `{"Version":"2012-10-17","Statement":[{"Effect":"Deny","Principal":"*","Action":"s3:PutObject","Resource":"arn:aws:s3:::mybucket/*"}]}`
	if IsAllowed(false, "s3:PutObject", "alice", "mybucket", "key", denyPolicy) {
		t.Error("IsAllowed(non-root, deny policy) should return false")
	}
}
