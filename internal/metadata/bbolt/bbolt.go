// Package bbolt implements metadata.Store using BoltDB (bbolt).
// All data is persisted in a single file. Bucket and object records are
// stored as JSON values keyed by name. Concurrent access is safe; bbolt
// serialises writes internally.
package bbolt

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	bolt "go.etcd.io/bbolt"

	"github.com/madhavkobal/sangraha/internal/metadata"
)

// Bucket names used internally by bbolt.
var (
	bucketsBucket   = []byte("buckets")
	objectsBucket   = []byte("objects")
	versionsBucket  = []byte("versions")
	multipartBucket = []byte("multipart")
	partsBucket     = []byte("parts")
	keysBucket      = []byte("keys")
)

// Store is a bbolt-backed metadata store.
type Store struct {
	db *bolt.DB
}

// Open opens (or creates) a bbolt database at path with a 5-second timeout.
func Open(path string) (*Store, error) {
	db, err := bolt.Open(path, 0o600, &bolt.Options{Timeout: 5 * time.Second})
	if err != nil {
		return nil, fmt.Errorf("bbolt open %s: %w", path, err)
	}
	if err = db.Update(func(tx *bolt.Tx) error {
		for _, name := range [][]byte{bucketsBucket, objectsBucket, versionsBucket, multipartBucket, partsBucket, keysBucket} {
			if _, cerr := tx.CreateBucketIfNotExists(name); cerr != nil {
				return fmt.Errorf("create bucket %s: %w", name, cerr)
			}
		}
		return nil
	}); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("bbolt init: %w", err)
	}
	return &Store{db: db}, nil
}

// Close closes the bbolt database.
func (s *Store) Close() error {
	return s.db.Close()
}

// --- Bucket operations ---

// PutBucket creates or overwrites a bucket record.
func (s *Store) PutBucket(_ context.Context, b metadata.BucketRecord) error {
	return s.put(bucketsBucket, b.Name, b)
}

// GetBucket returns the bucket record for name.
func (s *Store) GetBucket(_ context.Context, name string) (metadata.BucketRecord, error) {
	var b metadata.BucketRecord
	err := s.get(bucketsBucket, name, &b)
	return b, err
}

// DeleteBucket removes the bucket record.
func (s *Store) DeleteBucket(_ context.Context, name string) error {
	return s.delete(bucketsBucket, name)
}

// ListBuckets returns all bucket records ordered by name.
func (s *Store) ListBuckets(_ context.Context) ([]metadata.BucketRecord, error) {
	var out []metadata.BucketRecord
	err := s.db.View(func(tx *bolt.Tx) error {
		bkt := tx.Bucket(bucketsBucket)
		return bkt.ForEach(func(_, v []byte) error {
			var rec metadata.BucketRecord
			if err := json.Unmarshal(v, &rec); err != nil {
				return fmt.Errorf("bbolt: unmarshal bucket: %w", err)
			}
			out = append(out, rec)
			return nil
		})
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

// BucketExists reports whether the named bucket record exists.
func (s *Store) BucketExists(_ context.Context, name string) (bool, error) {
	exists := false
	err := s.db.View(func(tx *bolt.Tx) error {
		bkt := tx.Bucket(bucketsBucket)
		exists = bkt.Get([]byte(name)) != nil
		return nil
	})
	return exists, err
}

// --- Object operations ---

// PutObject creates or overwrites an object record.
func (s *Store) PutObject(_ context.Context, o metadata.ObjectRecord) error {
	return s.put(objectsBucket, o.Bucket+"\x00"+o.Key, o)
}

// GetObject returns the object record for bucket/key.
func (s *Store) GetObject(_ context.Context, bucket, key string) (metadata.ObjectRecord, error) {
	var o metadata.ObjectRecord
	err := s.get(objectsBucket, bucket+"\x00"+key, &o)
	return o, err
}

// DeleteObject removes the object record.
func (s *Store) DeleteObject(_ context.Context, bucket, key string) error {
	return s.delete(objectsBucket, bucket+"\x00"+key)
}

// ObjectExists reports whether an object record exists.
func (s *Store) ObjectExists(_ context.Context, bucket, key string) (bool, error) {
	exists := false
	err := s.db.View(func(tx *bolt.Tx) error {
		bkt := tx.Bucket(objectsBucket)
		exists = bkt.Get([]byte(bucket+"\x00"+key)) != nil
		return nil
	})
	return exists, err
}

// ListObjects lists objects in bucket filtered by opts.
func (s *Store) ListObjects(_ context.Context, bucket string, opts metadata.ListOptions) ([]metadata.ObjectRecord, []string, error) {
	if opts.MaxKeys <= 0 {
		opts.MaxKeys = 1000
	}
	prefix := bucket + "\x00" + opts.Prefix
	startAfterKey := ""
	if opts.ContinuationToken != "" {
		startAfterKey = opts.ContinuationToken
	} else if opts.StartAfter != "" {
		startAfterKey = bucket + "\x00" + opts.StartAfter
	}

	var records []metadata.ObjectRecord
	prefixSet := map[string]struct{}{}

	err := s.db.View(func(tx *bolt.Tx) error {
		bkt := tx.Bucket(objectsBucket)
		c := bkt.Cursor()

		var k, v []byte
		if startAfterKey != "" {
			k, v = c.Seek([]byte(startAfterKey + "\x01")) // byte after
		} else {
			k, v = c.Seek([]byte(prefix))
		}

		for ; k != nil; k, v = c.Next() {
			ks := string(k)
			if !strings.HasPrefix(ks, prefix) {
				break
			}
			objectKey := strings.TrimPrefix(ks, bucket+"\x00")

			// Apply delimiter grouping.
			if opts.Delimiter != "" {
				afterPrefix := strings.TrimPrefix(objectKey, opts.Prefix)
				if idx := strings.Index(afterPrefix, opts.Delimiter); idx >= 0 {
					cp := opts.Prefix + afterPrefix[:idx+len(opts.Delimiter)]
					prefixSet[cp] = struct{}{}
					continue
				}
			}

			if len(records)+len(prefixSet) >= opts.MaxKeys {
				break
			}

			var rec metadata.ObjectRecord
			if err := json.Unmarshal(v, &rec); err != nil {
				return fmt.Errorf("bbolt: unmarshal object: %w", err)
			}
			records = append(records, rec)
		}
		return nil
	})
	if err != nil {
		return nil, nil, err
	}

	commonPrefixes := make([]string, 0, len(prefixSet))
	for cp := range prefixSet {
		commonPrefixes = append(commonPrefixes, cp)
	}
	sort.Strings(commonPrefixes)
	return records, commonPrefixes, nil
}

// UpdateBucketStats atomically adjusts bucket stats.
func (s *Store) UpdateBucketStats(_ context.Context, bucket string, deltaCount, deltaBytes int64) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		bkt := tx.Bucket(bucketsBucket)
		v := bkt.Get([]byte(bucket))
		if v == nil {
			return &metadata.ErrNotFound{Kind: "bucket", Name: bucket}
		}
		var rec metadata.BucketRecord
		if err := json.Unmarshal(v, &rec); err != nil {
			return fmt.Errorf("bbolt: unmarshal bucket: %w", err)
		}
		rec.ObjectCount += deltaCount
		rec.TotalBytes += deltaBytes
		data, err := json.Marshal(rec)
		if err != nil {
			return fmt.Errorf("bbolt: marshal bucket: %w", err)
		}
		return bkt.Put([]byte(bucket), data)
	})
}

// --- Versioning operations ---

// versionKey returns the bbolt key for a version record.
// Format: bucket\x00key\x00versionID
func versionKey(bucket, key, versionID string) string {
	return bucket + "\x00" + key + "\x00" + versionID
}

// versionPrefix returns the prefix for all versions of bucket/key.
func versionPrefix(bucket, key string) string {
	return bucket + "\x00" + key + "\x00"
}

// PutVersion stores a version record.
func (s *Store) PutVersion(_ context.Context, v metadata.VersionRecord) error {
	return s.put(versionsBucket, versionKey(v.Bucket, v.Key, v.VersionID), v)
}

// GetVersion returns a specific version record.
func (s *Store) GetVersion(_ context.Context, bucket, key, versionID string) (metadata.VersionRecord, error) {
	var v metadata.VersionRecord
	err := s.get(versionsBucket, versionKey(bucket, key, versionID), &v)
	return v, err
}

// ListVersions returns all versions of bucket/key, newest first.
func (s *Store) ListVersions(_ context.Context, bucket, key string) ([]metadata.VersionRecord, error) {
	prefix := versionPrefix(bucket, key)
	var out []metadata.VersionRecord
	err := s.db.View(func(tx *bolt.Tx) error {
		bkt := tx.Bucket(versionsBucket)
		c := bkt.Cursor()
		for k, v := c.Seek([]byte(prefix)); k != nil && strings.HasPrefix(string(k), prefix); k, v = c.Next() {
			var rec metadata.VersionRecord
			if err := json.Unmarshal(v, &rec); err != nil {
				return fmt.Errorf("bbolt: unmarshal version: %w", err)
			}
			out = append(out, rec)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].LastModified.After(out[j].LastModified)
	})
	return out, nil
}

// ListBucketVersions returns all versions in a bucket for ListObjectVersions.
func (s *Store) ListBucketVersions(_ context.Context, bucket string, opts metadata.ListOptions) ([]metadata.VersionRecord, error) {
	if opts.MaxKeys <= 0 {
		opts.MaxKeys = 1000
	}
	prefix := bucket + "\x00" + opts.Prefix
	var out []metadata.VersionRecord
	err := s.db.View(func(tx *bolt.Tx) error {
		bkt := tx.Bucket(versionsBucket)
		c := bkt.Cursor()
		for k, v := c.Seek([]byte(prefix)); k != nil && strings.HasPrefix(string(k), prefix); k, v = c.Next() {
			var rec metadata.VersionRecord
			if err := json.Unmarshal(v, &rec); err != nil {
				return fmt.Errorf("bbolt: unmarshal version: %w", err)
			}
			out = append(out, rec)
			if len(out) >= opts.MaxKeys {
				break
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Key != out[j].Key {
			return out[i].Key < out[j].Key
		}
		return out[i].LastModified.After(out[j].LastModified)
	})
	return out, nil
}

// DeleteVersion removes a specific version record.
func (s *Store) DeleteVersion(_ context.Context, bucket, key, versionID string) error {
	return s.delete(versionsBucket, versionKey(bucket, key, versionID))
}

// MarkVersionsNotLatest marks all existing versions of bucket/key as not latest.
func (s *Store) MarkVersionsNotLatest(_ context.Context, bucket, key string) error {
	prefix := versionPrefix(bucket, key)
	return s.db.Update(func(tx *bolt.Tx) error {
		bkt := tx.Bucket(versionsBucket)
		c := bkt.Cursor()
		var updates []struct {
			key []byte
			val []byte
		}
		for k, v := c.Seek([]byte(prefix)); k != nil && strings.HasPrefix(string(k), prefix); k, v = c.Next() {
			var rec metadata.VersionRecord
			if err := json.Unmarshal(v, &rec); err != nil {
				return fmt.Errorf("bbolt: unmarshal version: %w", err)
			}
			if rec.IsLatest {
				rec.IsLatest = false
				data, err := json.Marshal(rec)
				if err != nil {
					return fmt.Errorf("bbolt: marshal version: %w", err)
				}
				keyCopy := make([]byte, len(k))
				copy(keyCopy, k)
				updates = append(updates, struct {
					key []byte
					val []byte
				}{keyCopy, data})
			}
		}
		for _, u := range updates {
			if err := bkt.Put(u.key, u.val); err != nil {
				return fmt.Errorf("bbolt: put version: %w", err)
			}
		}
		return nil
	})
}

// --- Multipart operations ---

// PutMultipart creates an in-progress upload record.
func (s *Store) PutMultipart(_ context.Context, m metadata.MultipartRecord) error {
	return s.put(multipartBucket, m.UploadID, m)
}

// GetMultipart returns the multipart record for uploadID.
func (s *Store) GetMultipart(_ context.Context, uploadID string) (metadata.MultipartRecord, error) {
	var m metadata.MultipartRecord
	err := s.get(multipartBucket, uploadID, &m)
	return m, err
}

// DeleteMultipart removes the multipart record.
func (s *Store) DeleteMultipart(_ context.Context, uploadID string) error {
	return s.delete(multipartBucket, uploadID)
}

// ListMultiparts returns all in-progress uploads for a bucket.
func (s *Store) ListMultiparts(_ context.Context, bucket string) ([]metadata.MultipartRecord, error) {
	var out []metadata.MultipartRecord
	err := s.db.View(func(tx *bolt.Tx) error {
		bkt := tx.Bucket(multipartBucket)
		return bkt.ForEach(func(_, v []byte) error {
			var rec metadata.MultipartRecord
			if err := json.Unmarshal(v, &rec); err != nil {
				return fmt.Errorf("bbolt: unmarshal multipart: %w", err)
			}
			if rec.Bucket == bucket {
				out = append(out, rec)
			}
			return nil
		})
	})
	return out, err
}

// PutPart records an uploaded part.
func (s *Store) PutPart(_ context.Context, p metadata.PartRecord) error {
	key := fmt.Sprintf("%s\x00%05d", p.UploadID, p.PartNumber)
	return s.put(partsBucket, key, p)
}

// ListParts returns all parts for the given upload, ordered by part number.
func (s *Store) ListParts(_ context.Context, uploadID string) ([]metadata.PartRecord, error) {
	prefix := uploadID + "\x00"
	var out []metadata.PartRecord
	err := s.db.View(func(tx *bolt.Tx) error {
		bkt := tx.Bucket(partsBucket)
		c := bkt.Cursor()
		for k, v := c.Seek([]byte(prefix)); k != nil && strings.HasPrefix(string(k), prefix); k, v = c.Next() {
			var rec metadata.PartRecord
			if err := json.Unmarshal(v, &rec); err != nil {
				return fmt.Errorf("bbolt: unmarshal part: %w", err)
			}
			out = append(out, rec)
		}
		return nil
	})
	return out, err
}

// DeleteParts removes all part records for the given upload.
func (s *Store) DeleteParts(_ context.Context, uploadID string) error {
	prefix := uploadID + "\x00"
	return s.db.Update(func(tx *bolt.Tx) error {
		bkt := tx.Bucket(partsBucket)
		c := bkt.Cursor()
		var toDelete [][]byte
		for k, _ := c.Seek([]byte(prefix)); k != nil && strings.HasPrefix(string(k), prefix); k, _ = c.Next() {
			toDelete = append(toDelete, append([]byte{}, k...))
		}
		for _, k := range toDelete {
			if err := bkt.Delete(k); err != nil {
				return fmt.Errorf("bbolt: delete part: %w", err)
			}
		}
		return nil
	})
}

// --- Access key operations ---

// PutAccessKey stores an access key record.
func (s *Store) PutAccessKey(_ context.Context, k metadata.AccessKeyRecord) error {
	return s.put(keysBucket, k.AccessKey, k)
}

// GetAccessKey returns the record for the given access key.
func (s *Store) GetAccessKey(_ context.Context, accessKey string) (metadata.AccessKeyRecord, error) {
	var k metadata.AccessKeyRecord
	err := s.get(keysBucket, accessKey, &k)
	return k, err
}

// DeleteAccessKey removes the access key record.
func (s *Store) DeleteAccessKey(_ context.Context, accessKey string) error {
	return s.delete(keysBucket, accessKey)
}

// ListAccessKeys returns all access key records.
func (s *Store) ListAccessKeys(_ context.Context) ([]metadata.AccessKeyRecord, error) {
	var out []metadata.AccessKeyRecord
	err := s.db.View(func(tx *bolt.Tx) error {
		bkt := tx.Bucket(keysBucket)
		return bkt.ForEach(func(_, v []byte) error {
			var rec metadata.AccessKeyRecord
			if err := json.Unmarshal(v, &rec); err != nil {
				return fmt.Errorf("bbolt: unmarshal key: %w", err)
			}
			out = append(out, rec)
			return nil
		})
	})
	return out, err
}

// --- Helpers ---

func (s *Store) put(bktName []byte, key string, val any) error {
	data, err := json.Marshal(val)
	if err != nil {
		return fmt.Errorf("bbolt put: marshal: %w", err)
	}
	return s.db.Update(func(tx *bolt.Tx) error {
		bkt := tx.Bucket(bktName)
		return bkt.Put([]byte(key), data)
	})
}

func (s *Store) get(bktName []byte, key string, dst any) error {
	return s.db.View(func(tx *bolt.Tx) error {
		bkt := tx.Bucket(bktName)
		v := bkt.Get([]byte(key))
		if v == nil {
			return &metadata.ErrNotFound{Kind: string(bktName), Name: key}
		}
		if err := json.Unmarshal(v, dst); err != nil {
			return fmt.Errorf("bbolt get: unmarshal: %w", err)
		}
		return nil
	})
}

func (s *Store) delete(bktName []byte, key string) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		bkt := tx.Bucket(bktName)
		return bkt.Delete([]byte(key))
	})
}
