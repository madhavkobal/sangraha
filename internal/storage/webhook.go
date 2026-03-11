package storage

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/madhavkobal/sangraha/internal/metadata"
)

// EventType categorises an S3-style event.
type EventType string

const (
	// EventObjectCreatedPut is fired after PutObject.
	EventObjectCreatedPut EventType = "s3:ObjectCreated:Put"
	// EventObjectCreatedMultipartCompleted is fired after CompleteMultipartUpload.
	EventObjectCreatedMultipartCompleted EventType = "s3:ObjectCreated:CompleteMultipartUpload"
	// EventObjectRemovedDelete is fired after DeleteObject.
	EventObjectRemovedDelete EventType = "s3:ObjectRemoved:Delete"
)

// NotificationEvent is the JSON payload delivered to webhooks.
type NotificationEvent struct {
	EventVersion string    `json:"eventVersion"`
	EventSource  string    `json:"eventSource"`
	EventTime    time.Time `json:"eventTime"`
	EventName    EventType `json:"eventName"`
	UserIdentity struct {
		PrincipalID string `json:"principalId"`
	} `json:"userIdentity"`
	RequestParameters struct {
		SourceIPAddress string `json:"sourceIPAddress"`
	} `json:"requestParameters"`
	S3 struct {
		SchemaVersion   string `json:"s3SchemaVersion"`
		ConfigurationID string `json:"configurationId"`
		Bucket          struct {
			Name string `json:"name"`
			Arn  string `json:"arn"`
		} `json:"bucket"`
		Object struct {
			Key  string `json:"key"`
			Size int64  `json:"size"`
			ETag string `json:"eTag"`
		} `json:"object"`
	} `json:"s3"`
}

// webhookTask is a single notification delivery job.
type webhookTask struct {
	event  NotificationEvent
	target metadata.WebhookTarget
}

// WebhookDispatcher delivers event notifications to configured webhook targets.
type WebhookDispatcher struct {
	queue  chan webhookTask
	wg     sync.WaitGroup
	stopCh chan struct{}
}

// NewWebhookDispatcher creates a dispatcher with the given concurrency.
func NewWebhookDispatcher(concurrency int) *WebhookDispatcher {
	if concurrency <= 0 {
		concurrency = 8
	}
	d := &WebhookDispatcher{
		queue:  make(chan webhookTask, 8192),
		stopCh: make(chan struct{}),
	}
	for i := 0; i < concurrency; i++ {
		d.wg.Add(1)
		go d.run()
	}
	return d
}

// Stop drains the queue and stops all workers.
func (d *WebhookDispatcher) Stop() {
	close(d.stopCh)
	d.wg.Wait()
}

// Dispatch enqueues notification delivery for all matching targets.
// It is non-blocking; tasks dropped when the queue is full are silently skipped.
func (d *WebhookDispatcher) Dispatch(cfg *metadata.NotificationConfig, event NotificationEvent) {
	if cfg == nil {
		return
	}
	for _, wt := range cfg.WebhookTargets {
		if !eventMatchesTarget(event.EventName, wt) {
			continue
		}
		select {
		case d.queue <- webhookTask{event: event, target: wt}:
		default:
		}
	}
}

func (d *WebhookDispatcher) run() {
	defer d.wg.Done()
	for {
		select {
		case <-d.stopCh:
			return
		case task := <-d.queue:
			deliver(task)
		}
	}
}

// deliver sends the event payload to the webhook URL with up to 3 attempts.
func deliver(task webhookTask) {
	payload, err := json.Marshal(task.event)
	if err != nil {
		return
	}
	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			time.Sleep(time.Duration(attempt*attempt) * time.Second)
		}
		if tryDeliver(task.target, payload) {
			return
		}
	}
}

func tryDeliver(target metadata.WebhookTarget, payload []byte) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, target.URL, bytes.NewReader(payload))
	if err != nil {
		return false
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Sangraha-Event-ID", uuid.New().String())
	if target.Secret != "" {
		sig := webhookSignature(target.Secret, payload)
		req.Header.Set("X-Sangraha-Signature", "sha256="+sig)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close() //nolint:errcheck
	return resp.StatusCode/100 == 2
}

func webhookSignature(secret string, payload []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	return hex.EncodeToString(mac.Sum(nil))
}

func eventMatchesTarget(event EventType, target metadata.WebhookTarget) bool {
	for _, pattern := range target.Events {
		if matchEventPattern(pattern, string(event)) {
			return true
		}
	}
	return false
}

// matchEventPattern matches an event against an S3-style pattern with '*' wildcard.
func matchEventPattern(pattern, event string) bool {
	if pattern == event {
		return true
	}
	if strings.HasSuffix(pattern, "*") {
		return strings.HasPrefix(event, strings.TrimSuffix(pattern, "*"))
	}
	return false
}

// buildEvent constructs a NotificationEvent for a PutObject result.
func buildObjectCreatedEvent(bucket, key, etag, owner string, size int64, eventName EventType) NotificationEvent {
	ev := NotificationEvent{
		EventVersion: "2.2",
		EventSource:  "sangraha:s3",
		EventTime:    time.Now().UTC(),
		EventName:    eventName,
	}
	ev.UserIdentity.PrincipalID = owner
	ev.S3.SchemaVersion = "1.0"
	ev.S3.Bucket.Name = bucket
	ev.S3.Bucket.Arn = fmt.Sprintf("arn:aws:s3:::%s", bucket)
	ev.S3.Object.Key = key
	ev.S3.Object.Size = size
	ev.S3.Object.ETag = etag
	return ev
}

// buildObjectRemovedEvent constructs a NotificationEvent for a DeleteObject.
func buildObjectRemovedEvent(bucket, key, owner string) NotificationEvent {
	ev := NotificationEvent{
		EventVersion: "2.2",
		EventSource:  "sangraha:s3",
		EventTime:    time.Now().UTC(),
		EventName:    EventObjectRemovedDelete,
	}
	ev.UserIdentity.PrincipalID = owner
	ev.S3.SchemaVersion = "1.0"
	ev.S3.Bucket.Name = bucket
	ev.S3.Bucket.Arn = fmt.Sprintf("arn:aws:s3:::%s", bucket)
	ev.S3.Object.Key = key
	return ev
}
