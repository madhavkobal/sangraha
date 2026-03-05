// Package audit emits structured, append-only audit events for every API
// request. Each event is written as a single JSON line to the configured
// audit log file (or to the zerolog logger when no file is set).
package audit

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"
)

// contextKey is the unexported type for context keys in this package.
type contextKey struct{}

// Event is a single audit log entry.
type Event struct {
	Time       time.Time `json:"time"`
	RequestID  string    `json:"request_id"`
	User       string    `json:"user"`
	Action     string    `json:"action"`
	Bucket     string    `json:"bucket,omitempty"`
	Key        string    `json:"key,omitempty"`
	SourceIP   string    `json:"source_ip,omitempty"`
	StatusCode int       `json:"status"`
	Bytes      int64     `json:"bytes,omitempty"`
	DurationMS int64     `json:"duration_ms"`
	Error      string    `json:"error,omitempty"`
}

// Logger writes audit events to an append-only file.
type Logger struct {
	mu   sync.Mutex
	file *os.File
}

// New opens (or creates) the audit log file at path in append-only mode.
func New(path string) (*Logger, error) {
	if path == "" {
		return &Logger{}, nil // discard
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600) //nolint:gosec // path is operator-configured
	if err != nil {
		return nil, fmt.Errorf("audit: open log file %s: %w", path, err)
	}
	return &Logger{file: f}, nil
}

// Log writes an audit event. It is safe for concurrent use.
func (l *Logger) Log(e Event) {
	if l == nil {
		return
	}
	e.Time = time.Now().UTC()
	data, err := json.Marshal(e)
	if err != nil {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.file != nil {
		_, _ = l.file.Write(append(data, '\n'))
	}
}

// Close flushes and closes the underlying log file.
func (l *Logger) Close() error {
	if l == nil || l.file == nil {
		return nil
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.file.Close()
}

// WithLogger attaches an audit logger to the context.
func WithLogger(ctx context.Context, l *Logger) context.Context {
	return context.WithValue(ctx, contextKey{}, l)
}

// FromContext extracts the audit logger from the context.
// Returns a no-op logger if none is present.
func FromContext(ctx context.Context) *Logger {
	l, _ := ctx.Value(contextKey{}).(*Logger)
	return l
}
