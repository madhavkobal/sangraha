package audit

import (
	"bufio"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestNewDiscardLogger(t *testing.T) {
	// Empty path returns a discard logger (no error).
	l, err := New("")
	if err != nil {
		t.Fatalf("New empty path: %v", err)
	}
	if l == nil {
		t.Fatal("New should return non-nil logger")
	}
	// Log should not panic on discard logger.
	l.Log(Event{User: "test", Action: "s3:PutObject"})
}

func TestNewFileLogger(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "audit.log")

	l, err := New(path)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	t.Cleanup(func() { _ = l.Close() })

	l.Log(Event{
		RequestID:  "req-1",
		User:       "alice",
		Action:     "s3:PutObject",
		Bucket:     "my-bucket",
		Key:        "test/key",
		StatusCode: 200,
		Bytes:      1024,
	})

	// Verify the event was written to the file.
	f, err := os.Open(path) //nolint:gosec
	if err != nil {
		t.Fatalf("open audit log: %v", err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	if !scanner.Scan() {
		t.Fatal("audit log should have at least one line")
	}

	var e Event
	if err := json.Unmarshal(scanner.Bytes(), &e); err != nil {
		t.Fatalf("unmarshal audit event: %v", err)
	}
	if e.User != "alice" {
		t.Errorf("user = %q; want alice", e.User)
	}
	if e.Action != "s3:PutObject" {
		t.Errorf("action = %q; want s3:PutObject", e.Action)
	}
	if e.Bytes != 1024 {
		t.Errorf("bytes = %d; want 1024", e.Bytes)
	}
	if e.Time.IsZero() {
		t.Error("time should be set on logged event")
	}
}

func TestLogConcurrentSafe(t *testing.T) {
	dir := t.TempDir()
	l, err := New(filepath.Join(dir, "audit.log"))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	t.Cleanup(func() { _ = l.Close() })

	done := make(chan struct{})
	for i := 0; i < 10; i++ {
		go func() {
			l.Log(Event{User: "concurrent-user", Action: "s3:GetObject"})
			done <- struct{}{}
		}()
	}
	for i := 0; i < 10; i++ {
		<-done
	}
}

func TestWithLoggerAndFromContext(t *testing.T) {
	l, _ := New("")
	ctx := WithLogger(context.Background(), l)
	got := FromContext(ctx)
	if got != l {
		t.Error("FromContext should return the logger stored by WithLogger")
	}
}

func TestFromContextMissing(t *testing.T) {
	got := FromContext(context.Background())
	if got != nil {
		t.Error("FromContext should return nil when no logger is in context")
	}
}

func TestNilLoggerLog(t *testing.T) {
	// Log on a nil *Logger should not panic.
	var l *Logger
	l.Log(Event{User: "test"})
}

func TestClose(t *testing.T) {
	dir := t.TempDir()
	l, err := New(filepath.Join(dir, "audit.log"))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if err := l.Close(); err != nil {
		t.Errorf("Close: %v", err)
	}
}
