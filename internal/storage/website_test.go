package storage

import (
	"context"
	"testing"

	"github.com/madhavkobal/sangraha/internal/metadata"
)

func TestSetAndGetBucketWebsite(t *testing.T) {
	eng := newTestEngine(t)
	ctx := context.Background()

	if err := eng.CreateBucket(ctx, "web-bucket", "owner", ""); err != nil {
		t.Fatal(err)
	}

	// No website config initially.
	cfg, err := eng.GetBucketWebsite(ctx, "web-bucket")
	if err != nil {
		t.Fatal(err)
	}
	if cfg != nil {
		t.Errorf("expected nil website config, got %+v", cfg)
	}

	// Set website config.
	want := &metadata.WebsiteConfig{
		IndexDocument: "index.html",
		ErrorDocument: "error.html",
	}
	if err = eng.SetBucketWebsite(ctx, "web-bucket", want); err != nil {
		t.Fatal(err)
	}
	got, err := eng.GetBucketWebsite(ctx, "web-bucket")
	if err != nil {
		t.Fatal(err)
	}
	if got == nil || got.IndexDocument != "index.html" || got.ErrorDocument != "error.html" {
		t.Errorf("unexpected website config: %+v", got)
	}

	// Remove website config.
	if err = eng.SetBucketWebsite(ctx, "web-bucket", nil); err != nil {
		t.Fatal(err)
	}
	cfg, err = eng.GetBucketWebsite(ctx, "web-bucket")
	if err != nil {
		t.Fatal(err)
	}
	if cfg != nil {
		t.Errorf("expected nil after removal, got %+v", cfg)
	}
}

func TestSetAndGetBucketNotification(t *testing.T) {
	eng := newTestEngine(t)
	ctx := context.Background()

	if err := eng.CreateBucket(ctx, "notif-bucket", "owner", ""); err != nil {
		t.Fatal(err)
	}

	// Set notification config with a webhook target.
	in := &metadata.NotificationConfig{
		WebhookTargets: []metadata.WebhookTarget{
			{ID: "wh1", URL: "https://example.com/hook", Events: []string{"s3:ObjectCreated:*"}},
		},
	}
	if err := eng.SetBucketNotification(ctx, "notif-bucket", in); err != nil {
		t.Fatal(err)
	}
	got, err := eng.GetBucketNotification(ctx, "notif-bucket")
	if err != nil {
		t.Fatal(err)
	}
	if got == nil || len(got.WebhookTargets) != 1 || got.WebhookTargets[0].URL != "https://example.com/hook" {
		t.Errorf("unexpected notification config: %+v", got)
	}

	// Remove.
	if err = eng.SetBucketNotification(ctx, "notif-bucket", nil); err != nil {
		t.Fatal(err)
	}
	got, err = eng.GetBucketNotification(ctx, "notif-bucket")
	if err != nil {
		t.Fatal(err)
	}
	if got != nil {
		t.Errorf("expected nil after removal, got %+v", got)
	}
}

func TestSetAndGetBucketReplication(t *testing.T) {
	eng := newTestEngine(t)
	ctx := context.Background()

	if err := eng.CreateBucket(ctx, "repl-bucket", "owner", ""); err != nil {
		t.Fatal(err)
	}

	// Set replication config.
	in := &metadata.ReplicationConfig{
		Rules: []metadata.ReplicationRule{
			{
				ID:     "rule1",
				Status: "Enabled",
				Destination: metadata.ReplicationDest{
					BucketARN: "arn:aws:s3:::dest-bucket",
					Endpoint:  "https://dest.example.com",
					AccessKey: "ak",
					SecretKey: "sk",
				},
			},
		},
	}
	if err := eng.SetBucketReplication(ctx, "repl-bucket", in); err != nil {
		t.Fatal(err)
	}
	got, err := eng.GetBucketReplication(ctx, "repl-bucket")
	if err != nil {
		t.Fatal(err)
	}
	if got == nil || len(got.Rules) != 1 || got.Rules[0].ID != "rule1" {
		t.Errorf("unexpected replication config: %+v", got)
	}

	// Remove.
	if err = eng.SetBucketReplication(ctx, "repl-bucket", nil); err != nil {
		t.Fatal(err)
	}
	got, err = eng.GetBucketReplication(ctx, "repl-bucket")
	if err != nil {
		t.Fatal(err)
	}
	if got != nil {
		t.Errorf("expected nil after removal, got %+v", got)
	}
}
