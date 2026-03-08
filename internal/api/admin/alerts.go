package admin

import (
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// AlertRule defines a threshold-based alerting rule.
type AlertRule struct {
	ID        string    `json:"id"`
	Metric    string    `json:"metric"`    // "disk_usage_pct" | "error_rate_pct" | "req_per_sec"
	Operator  string    `json:"operator"`  // "gt" | "lt" | "gte" | "lte"
	Threshold float64   `json:"threshold"` // numeric threshold value
	Label     string    `json:"label"`     // human-readable description
	CreatedAt time.Time `json:"created_at"`
}

// AlertEvent records a single alert firing.
type AlertEvent struct {
	ID         string     `json:"id"`
	RuleID     string     `json:"rule_id"`
	RuleLabel  string     `json:"rule_label"`
	Metric     string     `json:"metric"`
	FiredAt    time.Time  `json:"fired_at"`
	Value      float64    `json:"value"`
	Threshold  float64    `json:"threshold"`
	Resolved   bool       `json:"resolved"`
	ResolvedAt *time.Time `json:"resolved_at,omitempty"`
}

const maxAlertHistory = 100

// alertHandler manages alert rules and history in memory.
type alertHandler struct {
	mu      sync.RWMutex
	rules   []AlertRule
	history []AlertEvent
}

// createAlertRuleRequest is the JSON body for POST /admin/v1/alerts.
type createAlertRuleRequest struct {
	Metric    string  `json:"metric"`
	Operator  string  `json:"operator"`
	Threshold float64 `json:"threshold"`
	Label     string  `json:"label"`
}

// listRules handles GET /admin/v1/alerts.
func (h *alertHandler) listRules(w http.ResponseWriter, r *http.Request) {
	h.mu.RLock()
	out := make([]AlertRule, len(h.rules))
	copy(out, h.rules)
	h.mu.RUnlock()
	writeJSON(w, http.StatusOK, out)
}

// createRule handles POST /admin/v1/alerts.
func (h *alertHandler) createRule(w http.ResponseWriter, r *http.Request) {
	var req createAlertRuleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}
	if err := validateAlertRule(req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	rule := AlertRule{
		ID:        uuid.NewString(),
		Metric:    req.Metric,
		Operator:  req.Operator,
		Threshold: req.Threshold,
		Label:     req.Label,
		CreatedAt: time.Now().UTC(),
	}
	h.mu.Lock()
	h.rules = append(h.rules, rule)
	h.mu.Unlock()
	writeJSON(w, http.StatusCreated, rule)
}

// deleteRule handles DELETE /admin/v1/alerts/{id}.
func (h *alertHandler) deleteRule(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	h.mu.Lock()
	defer h.mu.Unlock()
	for i, rule := range h.rules {
		if rule.ID == id {
			h.rules = append(h.rules[:i], h.rules[i+1:]...)
			w.WriteHeader(http.StatusNoContent)
			return
		}
	}
	writeJSON(w, http.StatusNotFound, map[string]string{"error": "alert rule not found"})
}

// listHistory handles GET /admin/v1/alerts/history.
func (h *alertHandler) listHistory(w http.ResponseWriter, r *http.Request) {
	h.mu.RLock()
	out := make([]AlertEvent, len(h.history))
	copy(out, h.history)
	h.mu.RUnlock()
	writeJSON(w, http.StatusOK, out)
}

// RecordEvent appends a fired alert event to the ring-buffer history.
// Safe for concurrent use.
func (h *alertHandler) RecordEvent(e AlertEvent) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if len(h.history) >= maxAlertHistory {
		h.history = h.history[1:]
	}
	h.history = append(h.history, e)
}

func validateAlertRule(req createAlertRuleRequest) error {
	validMetrics := map[string]bool{
		"disk_usage_pct": true,
		"error_rate_pct": true,
		"req_per_sec":    true,
	}
	validOps := map[string]bool{"gt": true, "lt": true, "gte": true, "lte": true}
	if !validMetrics[req.Metric] {
		return &invalidAlertError{"metric must be one of: disk_usage_pct, error_rate_pct, req_per_sec"}
	}
	if !validOps[req.Operator] {
		return &invalidAlertError{"operator must be one of: gt, lt, gte, lte"}
	}
	if req.Label == "" {
		return &invalidAlertError{"label is required"}
	}
	return nil
}

type invalidAlertError struct{ msg string }

func (e *invalidAlertError) Error() string { return e.msg }
