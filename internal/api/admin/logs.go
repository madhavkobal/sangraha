package admin

import (
	"fmt"
	"net/http"
	"sync"
	"time"
)

// logBroadcaster maintains a set of SSE subscribers and broadcasts log lines to them.
var logBroadcaster = &broadcaster{subs: map[chan string]struct{}{}}

// broadcaster dispatches log lines to all active SSE subscribers.
type broadcaster struct {
	mu   sync.Mutex
	subs map[chan string]struct{}
}

// subscribe adds a new subscriber channel and returns it.
func (b *broadcaster) subscribe() chan string {
	ch := make(chan string, 64)
	b.mu.Lock()
	b.subs[ch] = struct{}{}
	b.mu.Unlock()
	return ch
}

// unsubscribe removes and closes a subscriber channel.
func (b *broadcaster) unsubscribe(ch chan string) {
	b.mu.Lock()
	delete(b.subs, ch)
	b.mu.Unlock()
	close(ch)
}

// Emit sends a log line to all subscribers, dropping slow consumers.
func (b *broadcaster) Emit(line string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for ch := range b.subs {
		select {
		case ch <- line:
		default:
			// drop slow consumer — do not block
		}
	}
}

// EmitLogLine publishes a structured log line to the SSE broadcaster.
// It is called by the zerolog hook wired in the server startup.
func EmitLogLine(line string) {
	logBroadcaster.Emit(line)
}

// handleLogStream handles GET /admin/v1/logs/stream.
// It streams structured log lines as Server-Sent Events.
// The optional ?level= query parameter filters by log level keyword.
func handleLogStream(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	levelFilter := r.URL.Query().Get("level")

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)

	// Send a comment to confirm connection.
	_, _ = fmt.Fprintf(w, ": connected\n\n")
	flusher.Flush()

	ch := logBroadcaster.subscribe()
	defer logBroadcaster.unsubscribe(ch)

	// Keepalive ticker so the client can detect dead connections.
	tick := time.NewTicker(15 * time.Second)
	defer tick.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case line, ok := <-ch:
			if !ok {
				return
			}
			if levelFilter != "" {
				// Simple string containment check on the JSON log line.
				if !containsLevel(line, levelFilter) {
					continue
				}
			}
			_, _ = fmt.Fprintf(w, "data: %s\n\n", line)
			flusher.Flush()
		case <-tick.C:
			_, _ = fmt.Fprintf(w, ": keepalive\n\n")
			flusher.Flush()
		}
	}
}

// containsLevel returns true if the JSON log line contains the given level value.
func containsLevel(line, level string) bool {
	// zerolog JSON lines contain `"level":"info"` etc.
	return len(line) > 0 && (level == "" || (len(line) > len(level) && findSubstring(line, `"level":"`+level+`"`)))
}

func findSubstring(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(s) > len(sub) && (s[:len(sub)] == sub || findSubstring(s[1:], sub)))
}
