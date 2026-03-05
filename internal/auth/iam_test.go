package auth

import "testing"

func TestIsAllowed(t *testing.T) {
	// Root users are allowed everything.
	if !IsAllowed(true, "s3:PutObject") {
		t.Error("IsAllowed(root) should return true")
	}
	if !IsAllowed(true, "s3:DeleteBucket") {
		t.Error("IsAllowed(root) should return true for any action")
	}
	// Non-root users are denied in Phase 1.
	if IsAllowed(false, "s3:PutObject") {
		t.Error("IsAllowed(non-root) should return false in Phase 1")
	}
	if IsAllowed(false, "") {
		t.Error("IsAllowed(non-root, empty) should return false")
	}
}
