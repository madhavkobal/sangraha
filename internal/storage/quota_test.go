package storage

import (
	"context"
	"strings"
	"testing"

	"github.com/madhavkobal/sangraha/internal/metadata"
)

func TestSetAndGetBucketQuota(t *testing.T) {
	eng := newTestEngine(t)
	ctx := context.Background()

	if err := eng.CreateBucket(ctx, "quota-bucket", "owner", "us-east-1"); err != nil {
		t.Fatal(err)
	}

	// No quota initially.
	q, err := eng.GetBucketQuota(ctx, "quota-bucket")
	if err != nil {
		t.Fatal(err)
	}
	if q != nil {
		t.Errorf("expected nil quota, got %+v", q)
	}

	// Set a quota.
	want := &metadata.BucketQuota{MaxSizeBytes: 1024, MaxObjects: 5}
	if err = eng.SetBucketQuota(ctx, "quota-bucket", want); err != nil {
		t.Fatal(err)
	}
	got, err := eng.GetBucketQuota(ctx, "quota-bucket")
	if err != nil {
		t.Fatal(err)
	}
	if got == nil || got.MaxSizeBytes != 1024 || got.MaxObjects != 5 {
		t.Errorf("unexpected quota: %+v", got)
	}

	// Remove quota.
	if err = eng.SetBucketQuota(ctx, "quota-bucket", nil); err != nil {
		t.Fatal(err)
	}
	q, err = eng.GetBucketQuota(ctx, "quota-bucket")
	if err != nil {
		t.Fatal(err)
	}
	if q != nil {
		t.Errorf("expected nil after removal, got %+v", q)
	}
}

func TestPutObjectRejectsWhenSizeQuotaExceeded(t *testing.T) {
	eng := newTestEngine(t)
	ctx := context.Background()

	if err := eng.CreateBucket(ctx, "size-quota", "owner", ""); err != nil {
		t.Fatal(err)
	}
	// Allow only 10 bytes.
	if err := eng.SetBucketQuota(ctx, "size-quota", &metadata.BucketQuota{MaxSizeBytes: 10}); err != nil {
		t.Fatal(err)
	}

	// 5 bytes — should succeed.
	_, err := eng.PutObject(ctx, PutObjectInput{
		Bucket: "size-quota",
		Key:    "small",
		Body:   strings.NewReader("hello"),
		Size:   5,
		Owner:  "owner",
	})
	if err != nil {
		t.Fatalf("expected success under quota, got: %v", err)
	}

	// 20 bytes — should fail with QuotaExceededError.
	_, err = eng.PutObject(ctx, PutObjectInput{
		Bucket: "size-quota",
		Key:    "big",
		Body:   strings.NewReader("01234567890123456789"),
		Size:   20,
		Owner:  "owner",
	})
	if err == nil {
		t.Fatal("expected quota error, got nil")
	}
	var qe *QuotaExceededError
	if !isErrorType(err, &qe) {
		t.Errorf("expected QuotaExceededError, got %T: %v", err, err)
	}
}

func TestPutObjectRejectsWhenObjectCountQuotaExceeded(t *testing.T) {
	eng := newTestEngine(t)
	ctx := context.Background()

	if err := eng.CreateBucket(ctx, "count-quota", "owner", ""); err != nil {
		t.Fatal(err)
	}
	if err := eng.SetBucketQuota(ctx, "count-quota", &metadata.BucketQuota{MaxObjects: 2}); err != nil {
		t.Fatal(err)
	}

	for i, key := range []string{"a", "b"} {
		_, err := eng.PutObject(ctx, PutObjectInput{
			Bucket: "count-quota",
			Key:    key,
			Body:   strings.NewReader("x"),
			Size:   1,
			Owner:  "owner",
		})
		if err != nil {
			t.Fatalf("object %d: %v", i, err)
		}
	}

	// Third object should be rejected.
	_, err := eng.PutObject(ctx, PutObjectInput{
		Bucket: "count-quota",
		Key:    "c",
		Body:   strings.NewReader("x"),
		Size:   1,
		Owner:  "owner",
	})
	if err == nil {
		t.Fatal("expected quota error on third object")
	}
	var qe *QuotaExceededError
	if !isErrorType(err, &qe) {
		t.Errorf("expected QuotaExceededError, got %T: %v", err, err)
	}
}

// isErrorType is a helper to check err matches the target type (like errors.As
// but for interface pointers in test code only).
func isErrorType(err error, target interface{}) bool {
	switch t := target.(type) {
	case **QuotaExceededError:
		var qe *QuotaExceededError
		if e, ok := err.(*QuotaExceededError); ok {
			*t = e
			return true
		}
		_ = qe
	}
	return false
}
