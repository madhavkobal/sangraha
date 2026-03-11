// Package backend defines the pluggable storage backend interface.
// Every concrete backend (localfs, badger, …) must satisfy this interface.
//
// CRITICAL: Never change this interface without a corresponding ADR in
// docs/architecture/. It is the contract between the storage engine and
// every backend implementation.
package backend

import (
	"context"
	"io"
	"time"
)

// ObjectInfo carries metadata about a stored object returned by Stat.
type ObjectInfo struct {
	// Size is the object's content length in bytes.
	Size int64
	// ModTime is the last-write time recorded by the backend.
	ModTime time.Time
}

// Backend is the pluggable storage interface. Implementations must be
// safe for concurrent use by multiple goroutines.
type Backend interface {
	// Write streams object data from r into bucket/key storage.
	// size is the declared content length; pass -1 if unknown.
	// Returns the number of bytes actually written, or an error.
	Write(ctx context.Context, bucket, key string, r io.Reader, size int64) (int64, error)

	// Read streams the object identified by bucket/key into w.
	Read(ctx context.Context, bucket, key string, w io.Writer) error

	// Delete removes the object identified by bucket/key.
	// Returns nil if the object does not exist (idempotent).
	Delete(ctx context.Context, bucket, key string) error

	// Exists reports whether the object identified by bucket/key is present.
	Exists(ctx context.Context, bucket, key string) (bool, error)

	// Stat returns size and modification time for the object.
	// Returns an error wrapping ErrNotFound if the object does not exist.
	Stat(ctx context.Context, bucket, key string) (ObjectInfo, error)
}

// ErrNotFound is returned by Stat and Read when the requested object does
// not exist in the backend.
type ErrNotFound struct {
	Bucket string
	Key    string
}

// Error implements the error interface.
func (e *ErrNotFound) Error() string {
	return "backend: object not found: " + e.Bucket + "/" + e.Key
}
