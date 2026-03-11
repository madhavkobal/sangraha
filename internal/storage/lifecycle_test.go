package storage

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/madhavkobal/sangraha/internal/metadata"
)

func TestSetGetDeleteLifecycleRules(t *testing.T) {
	ctx := context.Background()
	eng := newTestEngine(t)

	bucket := "lc-bucket"
	if err := eng.CreateBucket(ctx, bucket, "root", ""); err != nil {
		t.Fatalf("create bucket: %v", err)
	}

	rules := []metadata.LifecycleRule{
		{
			ID:             "expire-30",
			Status:         "Enabled",
			ExpirationDays: 30,
			Filter:         metadata.LifecycleFilter{Prefix: "logs/"},
		},
		{
			ID:                           "abort-multipart-7",
			Status:                       "Enabled",
			AbortIncompleteMultipartDays: 7,
		},
	}

	if err := eng.SetLifecycleRules(ctx, bucket, rules); err != nil {
		t.Fatalf("SetLifecycleRules: %v", err)
	}

	got, err := eng.GetLifecycleRules(ctx, bucket)
	if err != nil {
		t.Fatalf("GetLifecycleRules: %v", err)
	}
	if len(got) != len(rules) {
		t.Fatalf("got %d rules; want %d", len(got), len(rules))
	}
	if got[0].ID != rules[0].ID {
		t.Errorf("rule[0].ID = %q; want %q", got[0].ID, rules[0].ID)
	}

	if delErr := eng.DeleteLifecycleRules(ctx, bucket); delErr != nil {
		t.Fatalf("DeleteLifecycleRules: %v", delErr)
	}
	after, err := eng.GetLifecycleRules(ctx, bucket)
	if err != nil {
		t.Fatalf("GetLifecycleRules after delete: %v", err)
	}
	if len(after) != 0 {
		t.Errorf("after delete: expected 0 rules, got %d", len(after))
	}
}

func TestApplyLifecycleExpiresObjects(t *testing.T) {
	ctx := context.Background()
	eng := newTestEngine(t)

	bucket := "lc-expire"
	if err := eng.CreateBucket(ctx, bucket, "root", ""); err != nil {
		t.Fatalf("create bucket: %v", err)
	}

	rules := []metadata.LifecycleRule{
		{
			ID:             "expire-1d",
			Status:         "Enabled",
			ExpirationDays: 1,
		},
	}
	if err := eng.SetLifecycleRules(ctx, bucket, rules); err != nil {
		t.Fatalf("SetLifecycleRules: %v", err)
	}

	key := "old-file.txt"
	if _, err := eng.PutObject(ctx, PutObjectInput{
		Bucket: bucket,
		Key:    key,
		Body:   strings.NewReader("data"),
		Size:   4,
		Owner:  "root",
	}); err != nil {
		t.Fatalf("PutObject: %v", err)
	}

	// Backdate the object metadata so the lifecycle rule considers it expired.
	rec, err := eng.meta.GetObject(ctx, bucket, key)
	if err != nil {
		t.Fatalf("GetObject meta: %v", err)
	}
	rec.LastModified = time.Now().UTC().Add(-48 * time.Hour)
	if putErr := eng.meta.PutObject(ctx, rec); putErr != nil {
		t.Fatalf("backdate PutObject meta: %v", putErr)
	}

	if lcErr := eng.ApplyLifecycle(ctx); lcErr != nil {
		t.Fatalf("ApplyLifecycle: %v", lcErr)
	}

	// Object should be deleted.
	_, err = eng.GetObject(ctx, GetObjectInput{Bucket: bucket, Key: key})
	if err == nil {
		t.Error("expected object to be deleted by lifecycle, but it still exists")
	}
}

func TestApplyLifecycleDisabledRuleSkipped(t *testing.T) {
	ctx := context.Background()
	eng := newTestEngine(t)

	bucket := "lc-disabled"
	if err := eng.CreateBucket(ctx, bucket, "root", ""); err != nil {
		t.Fatalf("create bucket: %v", err)
	}

	rules := []metadata.LifecycleRule{
		{
			ID:             "disabled-rule",
			Status:         "Disabled",
			ExpirationDays: 1,
		},
	}
	if err := eng.SetLifecycleRules(ctx, bucket, rules); err != nil {
		t.Fatalf("SetLifecycleRules: %v", err)
	}

	key := "keep-me.txt"
	if _, err := eng.PutObject(ctx, PutObjectInput{
		Bucket: bucket,
		Key:    key,
		Body:   strings.NewReader("keep"),
		Size:   4,
		Owner:  "root",
	}); err != nil {
		t.Fatalf("PutObject: %v", err)
	}

	// Backdate.
	rec, err := eng.meta.GetObject(ctx, bucket, key)
	if err != nil {
		t.Fatalf("get meta: %v", err)
	}
	rec.LastModified = time.Now().UTC().Add(-48 * time.Hour)
	if putErr := eng.meta.PutObject(ctx, rec); putErr != nil {
		t.Fatalf("backdate: %v", putErr)
	}

	if lcErr := eng.ApplyLifecycle(ctx); lcErr != nil {
		t.Fatalf("ApplyLifecycle: %v", lcErr)
	}

	// Object must still exist — disabled rule must not be applied.
	getOut, err := eng.GetObject(ctx, GetObjectInput{Bucket: bucket, Key: key})
	if err != nil {
		t.Errorf("object was deleted by a disabled rule: %v", err)
		return
	}
	_ = getOut.Body.Close()
}

func TestObjectMatchesFilter(t *testing.T) {
	obj := metadata.ObjectRecord{
		Key:  "logs/app.log",
		Tags: map[string]string{"env": "prod", "team": "sre"},
	}
	tests := []struct {
		name   string
		filter metadata.LifecycleFilter
		want   bool
	}{
		{"no filter", metadata.LifecycleFilter{}, true},
		{"tag match", metadata.LifecycleFilter{Tags: map[string]string{"env": "prod"}}, true},
		{"tag no match", metadata.LifecycleFilter{Tags: map[string]string{"env": "dev"}}, false},
		{"multi tag match", metadata.LifecycleFilter{Tags: map[string]string{"env": "prod", "team": "sre"}}, true},
		{"multi tag partial no match", metadata.LifecycleFilter{Tags: map[string]string{"env": "prod", "team": "devops"}}, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := objectMatchesFilter(obj, tc.filter); got != tc.want {
				t.Errorf("objectMatchesFilter = %v; want %v", got, tc.want)
			}
		})
	}
}
