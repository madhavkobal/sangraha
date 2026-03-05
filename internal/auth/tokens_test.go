package auth

import (
	"context"
	"path/filepath"
	"testing"

	bboltstore "github.com/madhavkobal/sangraha/internal/metadata/bbolt"
)

func openTestKeyStore(t *testing.T) *KeyStore {
	t.Helper()
	s, err := bboltstore.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return NewKeyStore(s)
}

func TestCreateKey(t *testing.T) {
	ks := openTestKeyStore(t)
	ctx := context.Background()

	ak, sk, err := ks.CreateKey(ctx, "alice", false)
	if err != nil {
		t.Fatalf("CreateKey: %v", err)
	}
	if len(ak) == 0 || len(sk) == 0 {
		t.Error("CreateKey returned empty access or secret key")
	}

	// Lookup should succeed.
	rec, err := ks.Lookup(ctx, ak)
	if err != nil {
		t.Fatalf("Lookup: %v", err)
	}
	if rec.Owner != "alice" {
		t.Errorf("owner = %q; want %q", rec.Owner, "alice")
	}
	if rec.IsRoot {
		t.Error("IsRoot should be false")
	}

	// VerifySecret should confirm the generated secret.
	if !VerifySecret(rec, sk) {
		t.Error("VerifySecret returned false for correct secret")
	}
	if VerifySecret(rec, "wrong-secret") {
		t.Error("VerifySecret returned true for wrong secret")
	}
}

func TestUpsertKey(t *testing.T) {
	ks := openTestKeyStore(t)
	ctx := context.Background()

	if err := ks.UpsertKey(ctx, "rootkey", "mysecret", "root", true); err != nil {
		t.Fatalf("UpsertKey: %v", err)
	}

	rec, err := ks.Lookup(ctx, "rootkey")
	if err != nil {
		t.Fatalf("Lookup after UpsertKey: %v", err)
	}
	if !rec.IsRoot {
		t.Error("IsRoot should be true for root key")
	}
	if !VerifySecret(rec, "mysecret") {
		t.Error("VerifySecret failed for upserted key")
	}

	// Upsert again with new secret — old secret should no longer work.
	if err := ks.UpsertKey(ctx, "rootkey", "newsecret", "root", true); err != nil {
		t.Fatalf("second UpsertKey: %v", err)
	}
	rec2, _ := ks.Lookup(ctx, "rootkey")
	if VerifySecret(rec2, "mysecret") {
		t.Error("old secret should not verify after re-upsert")
	}
	if !VerifySecret(rec2, "newsecret") {
		t.Error("new secret should verify after re-upsert")
	}
}

func TestDeleteKey(t *testing.T) {
	ks := openTestKeyStore(t)
	ctx := context.Background()

	ak, _, err := ks.CreateKey(ctx, "bob", false)
	if err != nil {
		t.Fatalf("CreateKey: %v", err)
	}

	err = ks.DeleteKey(ctx, ak)
	if err != nil {
		t.Fatalf("DeleteKey: %v", err)
	}

	_, err = ks.Lookup(ctx, ak)
	if err == nil {
		t.Error("Lookup after DeleteKey should return error")
	}
}

func TestListKeys(t *testing.T) {
	ks := openTestKeyStore(t)
	ctx := context.Background()

	_, _, _ = ks.CreateKey(ctx, "user1", false)
	_, _, _ = ks.CreateKey(ctx, "user2", false)
	_ = ks.UpsertKey(ctx, "root", "secret", "root", true)

	keys, err := ks.ListKeys(ctx)
	if err != nil {
		t.Fatalf("ListKeys: %v", err)
	}
	if len(keys) != 3 {
		t.Errorf("got %d keys; want 3", len(keys))
	}
}

func TestLookupMissing(t *testing.T) {
	ks := openTestKeyStore(t)
	ctx := context.Background()

	_, err := ks.Lookup(ctx, "nonexistent-key")
	if err == nil {
		t.Error("Lookup of missing key should return error")
	}
}
