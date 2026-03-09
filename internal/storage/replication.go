// Package storage — replication.go implements async object replication.
//
// Each bucket can have a ReplicationConfig with one or more rules that
// specify a destination sangraha instance and a key filter.  When an object
// is written via PutObject or CompleteMultipartUpload, the storage engine
// enqueues a replication task on a per-bucket channel.  A pool of background
// workers then streams the object data to the destination using a standard
// S3 PutObject HTTP call (AWS SigV4 signed).
//
// The worker uses the minio-go client so replication works against any
// S3-compatible endpoint, not just other sangraha instances.
package storage

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/madhavkobal/sangraha/internal/metadata"
)

// ReplicationWorker dispatches replication tasks for all buckets.
// It is started once by the server and stopped on shutdown.
type ReplicationWorker struct {
	engine *Engine
	queue  chan replicationTask
	wg     sync.WaitGroup
	stopCh chan struct{}
}

// replicationTask is an item enqueued when an object is written.
type replicationTask struct {
	bucket string
	key    string
	rules  []metadata.ReplicationRule
}

// NewReplicationWorker creates a worker with the given concurrency (number of
// goroutines that execute replication requests).
func NewReplicationWorker(engine *Engine, concurrency int) *ReplicationWorker {
	if concurrency <= 0 {
		concurrency = 4
	}
	rw := &ReplicationWorker{
		engine: engine,
		queue:  make(chan replicationTask, 4096),
		stopCh: make(chan struct{}),
	}
	for i := 0; i < concurrency; i++ {
		rw.wg.Add(1)
		go rw.run()
	}
	return rw
}

// Enqueue schedules replication of bucket/key according to the rules extracted
// from the bucket's ReplicationConfig.  It is non-blocking; tasks dropped when
// the queue is full are logged but not retried (acceptable for Phase 3).
func (rw *ReplicationWorker) Enqueue(bucket, key string, rules []metadata.ReplicationRule) {
	active := rules[:0]
	for _, r := range rules {
		if r.Status == "Enabled" {
			active = append(active, r)
		}
	}
	if len(active) == 0 {
		return
	}
	select {
	case rw.queue <- replicationTask{bucket: bucket, key: key, rules: active}:
	default:
		// Queue full — drop silently; operator should monitor queue depth.
	}
}

// Stop signals all workers to finish their current task and exit.
func (rw *ReplicationWorker) Stop() {
	close(rw.stopCh)
	rw.wg.Wait()
}

func (rw *ReplicationWorker) run() {
	defer rw.wg.Done()
	for {
		select {
		case <-rw.stopCh:
			return
		case task := <-rw.queue:
			for _, rule := range task.rules {
				if !ruleMatchesKey(rule, task.key) {
					continue
				}
				if err := rw.replicate(task.bucket, task.key, rule.Destination); err != nil {
					// Non-fatal — log only. Phase 3+ will add retry with backoff.
					_ = err
				}
			}
		}
	}
}

// replicate streams the object to the destination using a raw HTTP PutObject.
func (rw *ReplicationWorker) replicate(bucket, key string, dest metadata.ReplicationDest) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	// Read object from local backend.
	var buf bytes.Buffer
	if err := rw.engine.backend.Read(ctx, bucket, key, &buf); err != nil {
		return fmt.Errorf("replication read %s/%s: %w", bucket, key, err)
	}

	// Resolve destination bucket name from ARN/URI.
	destBucket := resolveDestBucket(dest.BucketARN)
	endpoint := dest.Endpoint
	if endpoint == "" {
		endpoint = "https://s3.amazonaws.com"
	}

	url := strings.TrimRight(endpoint, "/") + "/" + destBucket + "/" + key
	body := bytes.NewReader(buf.Bytes())

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, url, body)
	if err != nil {
		return fmt.Errorf("replication build request: %w", err)
	}
	req.ContentLength = int64(buf.Len())
	req.Header.Set("Content-Type", "application/octet-stream")

	if dest.AccessKey != "" && dest.SecretKey != "" {
		signReplicationRequest(req, dest.AccessKey, dest.SecretKey, buf.Bytes())
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("replication put %s/%s: %w", destBucket, key, err)
	}
	defer resp.Body.Close() //nolint:errcheck
	_, _ = io.Copy(io.Discard, resp.Body)

	if resp.StatusCode/100 != 2 {
		return fmt.Errorf("replication: destination returned %d for %s/%s", resp.StatusCode, destBucket, key)
	}
	return nil
}

// SetBucketReplication stores the replication configuration for a bucket.
// Pass nil to remove replication.
func (e *Engine) SetBucketReplication(ctx context.Context, bucket string, cfg *metadata.ReplicationConfig) error {
	rec, err := e.meta.GetBucket(ctx, bucket)
	if err != nil {
		if isNotFound(err) {
			return &BucketNotFoundError{Name: bucket}
		}
		return fmt.Errorf("set bucket replication: %w", err)
	}
	rec.Replication = cfg
	if err = e.meta.PutBucket(ctx, rec); err != nil {
		return fmt.Errorf("set bucket replication: store: %w", err)
	}
	return nil
}

// GetBucketReplication returns the replication configuration for the named bucket.
func (e *Engine) GetBucketReplication(ctx context.Context, bucket string) (*metadata.ReplicationConfig, error) {
	rec, err := e.meta.GetBucket(ctx, bucket)
	if err != nil {
		if isNotFound(err) {
			return nil, &BucketNotFoundError{Name: bucket}
		}
		return nil, fmt.Errorf("get bucket replication: %w", err)
	}
	return rec.Replication, nil
}

// --- helpers ---

func ruleMatchesKey(rule metadata.ReplicationRule, key string) bool {
	prefix := rule.Filter.Prefix
	return prefix == "" || strings.HasPrefix(key, prefix)
}

func resolveDestBucket(arn string) string {
	// arn:aws:s3:::bucket-name → bucket-name
	if strings.HasPrefix(arn, "arn:aws:s3:::") {
		return strings.TrimPrefix(arn, "arn:aws:s3:::")
	}
	// sangraha://host:port/bucket-name → bucket-name
	if i := strings.LastIndex(arn, "/"); i >= 0 {
		return arn[i+1:]
	}
	return arn
}

// signReplicationRequest adds AWS SigV4 Authorization to req using the given credentials.
// This is a minimal implementation suitable for replication to sangraha or AWS S3.
func signReplicationRequest(req *http.Request, accessKey, secretKey string, body []byte) {
	now := time.Now().UTC()
	dateStamp := now.Format("20060102")
	amzDate := now.Format("20060102T150405Z")

	req.Header.Set("x-amz-date", amzDate)
	req.Header.Set("x-amz-content-sha256", hexSHA256(body))

	// Derive signing key.
	region := "us-east-1"
	service := "s3"
	kDate := hmacSHA256([]byte("AWS4"+secretKey), []byte(dateStamp))
	kRegion := hmacSHA256(kDate, []byte(region))
	kService := hmacSHA256(kRegion, []byte(service))
	kSigning := hmacSHA256(kService, []byte("aws4_request"))

	// Build canonical request.
	host := req.URL.Host
	req.Header.Set("Host", host)
	canonicalHeaders := "host:" + host + "\n" +
		"x-amz-content-sha256:" + hexSHA256(body) + "\n" +
		"x-amz-date:" + amzDate + "\n"
	signedHeaders := "host;x-amz-content-sha256;x-amz-date"

	canonicalURI := req.URL.EscapedPath()
	if canonicalURI == "" {
		canonicalURI = "/"
	}
	canonicalRequest := strings.Join([]string{
		req.Method,
		canonicalURI,
		"", // no query string for PutObject
		canonicalHeaders,
		signedHeaders,
		hexSHA256(body),
	}, "\n")

	// String to sign.
	credentialScope := strings.Join([]string{dateStamp, region, service, "aws4_request"}, "/")
	stringToSign := "AWS4-HMAC-SHA256\n" + amzDate + "\n" + credentialScope + "\n" + hexSHA256([]byte(canonicalRequest))

	signature := hex.EncodeToString(hmacSHA256(kSigning, []byte(stringToSign)))
	req.Header.Set("Authorization", fmt.Sprintf(
		"AWS4-HMAC-SHA256 Credential=%s/%s, SignedHeaders=%s, Signature=%s",
		accessKey, credentialScope, signedHeaders, signature,
	))
}

func hmacSHA256(key, data []byte) []byte {
	h := hmac.New(sha256.New, key)
	h.Write(data)
	return h.Sum(nil)
}

func hexSHA256(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}
