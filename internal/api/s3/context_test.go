package s3

import (
	"context"
	"testing"

	"github.com/madhavkobal/sangraha/internal/auth"
)

func TestWithIdentityAndFromContext(t *testing.T) {
	id := auth.VerifiedIdentity{AccessKey: "AKIATEST", Owner: "alice", IsRoot: true}
	ctx := withIdentity(context.Background(), id)
	got := identityFromContext(ctx)
	if got.AccessKey != id.AccessKey || got.Owner != id.Owner {
		t.Errorf("identityFromContext = %+v; want %+v", got, id)
	}
}

func TestIdentityFromContextMissing(t *testing.T) {
	got := identityFromContext(context.Background())
	if got.AccessKey != "" {
		t.Errorf("identityFromContext on empty ctx = %+v; want zero value", got)
	}
}

func TestWithRequestIDAndFromContext(t *testing.T) {
	ctx := withRequestID(context.Background(), "req-123")
	got := requestIDFromContext(ctx)
	if got != "req-123" {
		t.Errorf("requestIDFromContext = %q; want req-123", got)
	}
}

func TestRequestIDFromContextMissing(t *testing.T) {
	got := requestIDFromContext(context.Background())
	if got != "" {
		t.Errorf("requestIDFromContext on empty ctx = %q; want empty", got)
	}
}
