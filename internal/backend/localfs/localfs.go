// Package localfs implements the backend.Backend interface using the local
// filesystem. Each object is stored as a regular file at:
//
//	<root>/<bucket>/<key>
//
// Keys that contain path separators are stored verbatim; the directory tree
// is created as needed. This means that "foo/bar/baz" inside bucket "b"
// is stored at <root>/b/foo/bar/baz.
package localfs

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/madhavkobal/sangraha/internal/backend"
)

// Backend is the local filesystem backend.
type Backend struct {
	root string
}

// New creates a new localfs Backend rooted at root. The directory is created
// if it does not exist.
func New(root string) (*Backend, error) {
	if err := os.MkdirAll(root, 0o700); err != nil {
		return nil, fmt.Errorf("localfs: create root %s: %w", root, err)
	}
	return &Backend{root: root}, nil
}

// Write streams r into the object file at <root>/<bucket>/<key>.
func (b *Backend) Write(_ context.Context, bucket, key string, r io.Reader, _ int64) (int64, error) {
	p, err := b.safePath(bucket, key)
	if err != nil {
		return 0, err
	}
	if err = os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		return 0, fmt.Errorf("localfs write: mkdir: %w", err)
	}
	f, err := os.CreateTemp(filepath.Dir(p), ".tmp-*")
	if err != nil {
		return 0, fmt.Errorf("localfs write: create temp: %w", err)
	}
	tmpName := f.Name()
	defer func() {
		// If we did not rename the temp file, clean it up.
		_ = os.Remove(tmpName)
	}()

	n, err := io.Copy(f, r)
	if err != nil {
		_ = f.Close()
		return 0, fmt.Errorf("localfs write: copy: %w", err)
	}
	if err = f.Sync(); err != nil {
		_ = f.Close()
		return 0, fmt.Errorf("localfs write: sync: %w", err)
	}
	if err = f.Close(); err != nil {
		return 0, fmt.Errorf("localfs write: close: %w", err)
	}
	if err = os.Rename(tmpName, p); err != nil { //nolint:gosec // G304: destination path validated by safePath()
		return 0, fmt.Errorf("localfs write: rename: %w", err)
	}
	return n, nil
}

// Read streams the object at <root>/<bucket>/<key> into w.
func (b *Backend) Read(_ context.Context, bucket, key string, w io.Writer) error {
	p, err := b.safePath(bucket, key)
	if err != nil {
		return err
	}
	f, err := os.Open(p) //nolint:gosec // path is validated by safePath
	if err != nil {
		if os.IsNotExist(err) {
			return &backend.ErrNotFound{Bucket: bucket, Key: key}
		}
		return fmt.Errorf("localfs read: open: %w", err)
	}
	defer func() { _ = f.Close() }()
	if _, err = io.Copy(w, f); err != nil {
		return fmt.Errorf("localfs read: copy: %w", err)
	}
	return nil
}

// ReadRange streams a byte range of the object into w.
func (b *Backend) ReadRange(_ context.Context, bucket, key string, w io.Writer, offset, length int64) error {
	p, err := b.safePath(bucket, key)
	if err != nil {
		return err
	}
	f, err := os.Open(p) //nolint:gosec // path is validated by safePath
	if err != nil {
		if os.IsNotExist(err) {
			return &backend.ErrNotFound{Bucket: bucket, Key: key}
		}
		return fmt.Errorf("localfs readrange: open: %w", err)
	}
	defer func() { _ = f.Close() }()
	if _, err = f.Seek(offset, io.SeekStart); err != nil {
		return fmt.Errorf("localfs readrange: seek: %w", err)
	}
	if _, err = io.CopyN(w, f, length); err != nil && err != io.EOF {
		return fmt.Errorf("localfs readrange: copy: %w", err)
	}
	return nil
}

// Delete removes the object file. Returns nil if the file is already absent.
func (b *Backend) Delete(_ context.Context, bucket, key string) error {
	p, err := b.safePath(bucket, key)
	if err != nil {
		return err
	}
	err = os.Remove(p)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("localfs delete: %w", err)
	}
	return nil
}

// Exists reports whether the object file is present.
func (b *Backend) Exists(_ context.Context, bucket, key string) (bool, error) {
	p, err := b.safePath(bucket, key)
	if err != nil {
		return false, err
	}
	_, err = os.Stat(p)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, fmt.Errorf("localfs exists: %w", err)
}

// Stat returns the size and modification time for an object file.
func (b *Backend) Stat(_ context.Context, bucket, key string) (backend.ObjectInfo, error) {
	p, err := b.safePath(bucket, key)
	if err != nil {
		return backend.ObjectInfo{}, err
	}
	fi, err := os.Stat(p)
	if err != nil {
		if os.IsNotExist(err) {
			return backend.ObjectInfo{}, &backend.ErrNotFound{Bucket: bucket, Key: key}
		}
		return backend.ObjectInfo{}, fmt.Errorf("localfs stat: %w", err)
	}
	return backend.ObjectInfo{Size: fi.Size(), ModTime: fi.ModTime().UTC()}, nil
}

// CreateBucketDir creates the directory for a new bucket.
func (b *Backend) CreateBucketDir(bucket string) error {
	if err := validateBucket(bucket); err != nil {
		return err
	}
	dir := filepath.Join(b.root, bucket)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("localfs: create bucket dir: %w", err)
	}
	return nil
}

// DeleteBucketDir removes the bucket directory. Fails if non-empty.
func (b *Backend) DeleteBucketDir(bucket string) error {
	if err := validateBucket(bucket); err != nil {
		return err
	}
	dir := filepath.Join(b.root, bucket)
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("localfs: read bucket dir: %w", err)
	}
	if len(entries) > 0 {
		return fmt.Errorf("localfs: bucket %q is not empty", bucket)
	}
	return os.Remove(dir)
}

// safePath joins root+bucket+key, ensuring no path traversal can escape root.
func (b *Backend) safePath(bucket, key string) (string, error) {
	if err := validateBucket(bucket); err != nil {
		return "", err
	}
	if err := validateKey(key); err != nil {
		return "", err
	}
	// Clean the key to remove any .. components.
	cleanKey := filepath.FromSlash(key)
	joined := filepath.Join(b.root, bucket, cleanKey)
	// Ensure the resulting path is still under root.
	if !strings.HasPrefix(joined, filepath.Clean(b.root)+string(filepath.Separator)) {
		return "", fmt.Errorf("localfs: path traversal detected in key %q", key)
	}
	return joined, nil
}

func validateBucket(bucket string) error {
	if bucket == "" {
		return fmt.Errorf("localfs: bucket name must not be empty")
	}
	if strings.ContainsAny(bucket, "/\\") {
		return fmt.Errorf("localfs: bucket name %q contains illegal characters", bucket)
	}
	return nil
}

func validateKey(key string) error {
	if key == "" {
		return fmt.Errorf("localfs: object key must not be empty")
	}
	// Reject keys that would traverse outside the bucket directory.
	cleaned := filepath.Clean(filepath.FromSlash(key))
	if strings.HasPrefix(cleaned, "..") {
		return fmt.Errorf("localfs: object key %q attempts path traversal", key)
	}
	return nil
}

// ModTime returns the modification time of the object (for tests / internal use).
func (b *Backend) ModTime(_ context.Context, bucket, key string) (time.Time, error) {
	info, err := b.Stat(nil, bucket, key) //nolint:staticcheck // context is unused in stat
	if err != nil {
		return time.Time{}, err
	}
	return info.ModTime, nil
}
