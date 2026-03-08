package admin

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"
)

// auditEntry mirrors audit.Event for decoding from the log file.
type auditEntry struct {
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

// auditFilter holds the parsed query parameters for audit log filtering.
type auditFilter struct {
	user     string
	bucket   string
	action   string
	fromTime time.Time
	toTime   time.Time
	limit    int
}

// matches reports whether the entry passes all filter conditions.
func (f *auditFilter) matches(e auditEntry) bool {
	if !f.fromTime.IsZero() && e.Time.Before(f.fromTime) {
		return false
	}
	if !f.toTime.IsZero() && e.Time.After(f.toTime) {
		return false
	}
	if f.user != "" && e.User != f.user {
		return false
	}
	if f.bucket != "" && e.Bucket != f.bucket {
		return false
	}
	if f.action != "" && e.Action != f.action {
		return false
	}
	return true
}

// parseAuditFilter reads query parameters and returns an auditFilter.
func parseAuditFilter(r *http.Request) auditFilter {
	q := r.URL.Query()
	f := auditFilter{
		user:   q.Get("user"),
		bucket: q.Get("bucket"),
		action: q.Get("action"),
		limit:  200,
	}
	if lStr := q.Get("limit"); lStr != "" {
		if n, err := parseIntClamped(lStr, 1, 1000); err == nil {
			f.limit = n
		}
	}
	if s := q.Get("from"); s != "" {
		f.fromTime, _ = time.Parse(time.RFC3339, s)
	}
	if s := q.Get("to"); s != "" {
		f.toTime, _ = time.Parse(time.RFC3339, s)
	}
	return f
}

// auditHandler serves the audit log query endpoint.
type auditHandler struct {
	auditLogPath string
}

// handleAuditQuery handles GET /admin/v1/audit.
// Query params: from (RFC3339), to (RFC3339), user, bucket, action, limit (default 200, max 1000).
func (h *auditHandler) handleAuditQuery(w http.ResponseWriter, r *http.Request) {
	if h.auditLogPath == "" {
		writeJSON(w, http.StatusOK, []auditEntry{})
		return
	}

	filter := parseAuditFilter(r)

	results, err := h.scanAuditLog(filter)
	if err != nil {
		if os.IsNotExist(err) {
			writeJSON(w, http.StatusOK, []auditEntry{})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "cannot open audit log"})
		return
	}

	writeJSON(w, http.StatusOK, results)
}

// scanAuditLog opens the audit log file and returns matching entries up to filter.limit.
func (h *auditHandler) scanAuditLog(filter auditFilter) ([]auditEntry, error) {
	f, err := os.Open(h.auditLogPath) //nolint:gosec // operator-configured path
	if err != nil {
		return nil, err
	}
	defer f.Close() //nolint:errcheck

	var results []auditEntry
	scanner := bufio.NewScanner(f)
	buf := make([]byte, 256*1024)
	scanner.Buffer(buf, len(buf))

	for scanner.Scan() {
		var e auditEntry
		if err2 := json.Unmarshal(scanner.Bytes(), &e); err2 != nil {
			continue
		}
		if filter.matches(e) {
			results = append(results, e)
		}
	}

	// Return the most recent entries up to limit.
	if len(results) > filter.limit {
		results = results[len(results)-filter.limit:]
	}
	return results, nil
}

func parseIntClamped(s string, min, max int) (int, error) {
	var n int
	_, err := fmt.Sscanf(s, "%d", &n)
	if err != nil {
		return 0, err
	}
	if n < min {
		n = min
	}
	if n > max {
		n = max
	}
	return n, nil
}
