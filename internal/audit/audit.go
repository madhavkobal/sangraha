// Package audit emits structured, append-only audit events for every API
// request. Each event is written as a single JSON line to the configured
// audit log file or forwarded to a syslog server. A path prefix of
// "syslog://" routes events to UDP syslog; any other non-empty path opens
// a local append-only file.
package audit

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"strings"
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

// Logger writes audit events to an append-only file or a syslog server.
type Logger struct {
	mu     sync.Mutex
	file   *os.File
	syslog net.Conn // non-nil when forwarding to UDP syslog
}

// New opens (or creates) the audit log destination described by path.
//
//   - "" → discard (no-op logger)
//   - "syslog://host:port" → UDP syslog forwarding
//   - any other value → local append-only file
func New(path string) (*Logger, error) {
	if path == "" {
		return &Logger{}, nil // discard
	}
	if strings.HasPrefix(path, "syslog://") {
		addr := strings.TrimPrefix(path, "syslog://")
		d := &net.Dialer{}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		conn, err := d.DialContext(ctx, "udp", addr)
		if err != nil {
			return nil, fmt.Errorf("audit: connect to syslog %s: %w", addr, err)
		}
		return &Logger{syslog: conn}, nil
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
	line := append(data, '\n')
	if l.syslog != nil {
		// RFC 5424-ish: prepend a minimal syslog priority header.
		msg := fmt.Sprintf("<14>sangraha audit: %s", line)
		_, _ = l.syslog.Write([]byte(msg))
		return
	}
	if l.file != nil {
		_, _ = l.file.Write(line)
	}
}

// Close flushes and closes the underlying log file or syslog connection.
func (l *Logger) Close() error {
	if l == nil {
		return nil
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.syslog != nil {
		return l.syslog.Close()
	}
	if l.file != nil {
		return l.file.Close()
	}
	return nil
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
