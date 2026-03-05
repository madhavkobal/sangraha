// Package auth implements AWS Signature Version 4 verification, IAM policy
// evaluation, and access key / secret key management.
package auth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/madhavkobal/sangraha/internal/metadata"
)

// bcryptCost is the work factor for bcrypt. 12 is the minimum mandated by
// the security requirements in CLAUDE.md.
const bcryptCost = 12

// KeyStore manages access key / secret key pairs stored in the metadata store.
type KeyStore struct {
	meta metadata.Store
}

// NewKeyStore creates a KeyStore backed by the given metadata store.
func NewKeyStore(meta metadata.Store) *KeyStore {
	return &KeyStore{meta: meta}
}

// CreateKey generates a new random access key + secret key pair, stores the
// bcrypt hash and the plaintext signing key, and returns the plaintext secret
// (shown once). The plaintext signing key is required for SigV4 verification.
func (ks *KeyStore) CreateKey(ctx context.Context, owner string, isRoot bool) (accessKey, secretKey string, err error) {
	accessKey, err = randomBase62(20)
	if err != nil {
		return "", "", fmt.Errorf("create key: generate access key: %w", err)
	}
	secretKey, err = randomBase62(40)
	if err != nil {
		return "", "", fmt.Errorf("create key: generate secret key: %w", err)
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(secretKey), bcryptCost)
	if err != nil {
		return "", "", fmt.Errorf("create key: bcrypt: %w", err)
	}
	rec := metadata.AccessKeyRecord{
		AccessKey:  accessKey,
		SecretHash: string(hash),
		SigningKey:  secretKey, // stored for SigV4 verification
		Owner:      owner,
		CreatedAt:  time.Now().UTC(),
		IsRoot:     isRoot,
	}
	if err = ks.meta.PutAccessKey(ctx, rec); err != nil {
		return "", "", fmt.Errorf("create key: store: %w", err)
	}
	return accessKey, secretKey, nil
}

// UpsertKey stores a key with a known secret (used during server init for the
// root key whose secret is supplied via environment variable).
func (ks *KeyStore) UpsertKey(ctx context.Context, accessKey, secretKey, owner string, isRoot bool) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(secretKey), bcryptCost)
	if err != nil {
		return fmt.Errorf("upsert key: bcrypt: %w", err)
	}
	rec := metadata.AccessKeyRecord{
		AccessKey:  accessKey,
		SecretHash: string(hash),
		SigningKey:  secretKey, // stored for SigV4 verification
		Owner:      owner,
		CreatedAt:  time.Now().UTC(),
		IsRoot:     isRoot,
	}
	return ks.meta.PutAccessKey(ctx, rec)
}

// Lookup returns the bcrypt hash for accessKey, or an error if not found.
func (ks *KeyStore) Lookup(ctx context.Context, accessKey string) (metadata.AccessKeyRecord, error) {
	rec, err := ks.meta.GetAccessKey(ctx, accessKey)
	if err != nil {
		return metadata.AccessKeyRecord{}, fmt.Errorf("lookup key %q: %w", accessKey, err)
	}
	return rec, nil
}

// VerifySecret checks the plaintext secret against the stored bcrypt hash.
func VerifySecret(rec metadata.AccessKeyRecord, secret string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(rec.SecretHash), []byte(secret))
	return err == nil
}

// DeleteKey removes an access key.
func (ks *KeyStore) DeleteKey(ctx context.Context, accessKey string) error {
	return ks.meta.DeleteAccessKey(ctx, accessKey)
}

// ListKeys returns all access key records.
func (ks *KeyStore) ListKeys(ctx context.Context) ([]metadata.AccessKeyRecord, error) {
	return ks.meta.ListAccessKeys(ctx)
}

// randomBase62 returns a cryptographically random string of n Base62 characters.
func randomBase62(n int) (string, error) {
	const alphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"
	buf := make([]byte, n)
	raw := make([]byte, n*2) // over-provision to avoid modulo bias
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("random bytes: %w", err)
	}
	for i := range buf {
		buf[i] = alphabet[int(raw[i])%len(alphabet)]
	}
	return string(buf), nil
}

// randomBytes returns a cryptographically random byte slice of length n.
func randomBytes(n int) ([]byte, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return nil, fmt.Errorf("random bytes: %w", err)
	}
	return b, nil
}

// randomBase64URL returns a URL-safe Base64-encoded random string of n bytes.
func randomBase64URL(n int) (string, error) {
	b, err := randomBytes(n)
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
