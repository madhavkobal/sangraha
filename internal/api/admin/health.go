// Package admin implements the non-S3 admin REST API (port 9001).
package admin

import (
	"encoding/json"
	"net/http"
	"time"
)

var startTime = time.Now()

// healthResponse is the body for /admin/v1/health.
type healthResponse struct {
	Status string `json:"status"`
}

// readyResponse is the body for /admin/v1/ready.
type readyResponse struct {
	Status string `json:"status"`
}

// infoResponse is the body for /admin/v1/info.
type infoResponse struct {
	Version   string `json:"version"`
	BuildTime string `json:"build_time"`
	UptimeSec int64  `json:"uptime_sec"`
}

// handleHealth returns 200 OK as long as the server is running.
func handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, healthResponse{Status: "ok"})
}

// handleReady returns 200 OK when the server has finished initialising.
func handleReady(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, readyResponse{Status: "ready"})
}

// handleInfo returns version and uptime information.
func handleInfo(version, buildTime string) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, infoResponse{
			Version:   version,
			BuildTime: buildTime,
			UptimeSec: int64(time.Since(startTime).Seconds()),
		})
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	body, err := json.Marshal(v)
	if err != nil {
		http.Error(w, `{"error":"internal server error"}`, http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write(body)
}
