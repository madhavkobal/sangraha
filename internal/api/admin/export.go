package admin

import (
	"encoding/json"
	"net/http"
	"sync"
	"time"
)

// exportState tracks an in-progress export or import operation.
type exportState struct {
	mu        sync.Mutex
	running   bool
	operation string // "export" or "import"
	progress  int    // 0–100
	startedAt time.Time
	doneAt    time.Time
	err       string
}

var globalExportState = &exportState{}

// exportResponse is the body for POST /admin/v1/export.
type exportResponse struct {
	Started   bool   `json:"started"`
	Operation string `json:"operation"`
	Message   string `json:"message"`
}

// exportStatusResponse is the body for GET /admin/v1/export/status.
type exportStatusResponse struct {
	Running   bool      `json:"running"`
	Operation string    `json:"operation,omitempty"`
	Progress  int       `json:"progress"`
	StartedAt time.Time `json:"started_at,omitempty"`
	DoneAt    time.Time `json:"done_at,omitempty"`
	Error     string    `json:"error,omitempty"`
}

// backupSchedule holds the configured backup schedule.
type backupSchedule struct {
	mu          sync.RWMutex
	CronExpr    string `json:"cron"`
	Destination string `json:"destination"`
	Enabled     bool   `json:"enabled"`
}

var globalBackupSchedule = &backupSchedule{
	CronExpr:    "",
	Destination: "",
	Enabled:     false,
}

func handleExport(w http.ResponseWriter, _ *http.Request) {
	globalExportState.mu.Lock()
	if globalExportState.running {
		globalExportState.mu.Unlock()
		writeJSON(w, http.StatusConflict, map[string]string{
			"error": "export or import already running",
		})
		return
	}
	globalExportState.running = true
	globalExportState.operation = "export"
	globalExportState.progress = 0
	globalExportState.startedAt = time.Now()
	globalExportState.err = ""
	globalExportState.mu.Unlock()

	go runExportSimulation("export")

	writeJSON(w, http.StatusAccepted, exportResponse{
		Started:   true,
		Operation: "export",
		Message:   "export started; poll GET /admin/v1/export/status for progress",
	})
}

func handleImport(w http.ResponseWriter, _ *http.Request) {
	globalExportState.mu.Lock()
	if globalExportState.running {
		globalExportState.mu.Unlock()
		writeJSON(w, http.StatusConflict, map[string]string{
			"error": "export or import already running",
		})
		return
	}
	globalExportState.running = true
	globalExportState.operation = "import"
	globalExportState.progress = 0
	globalExportState.startedAt = time.Now()
	globalExportState.err = ""
	globalExportState.mu.Unlock()

	go runExportSimulation("import")

	writeJSON(w, http.StatusAccepted, exportResponse{
		Started:   true,
		Operation: "import",
		Message:   "import started; poll GET /admin/v1/export/status for progress",
	})
}

func handleExportStatus(w http.ResponseWriter, _ *http.Request) {
	globalExportState.mu.Lock()
	resp := exportStatusResponse{
		Running:   globalExportState.running,
		Operation: globalExportState.operation,
		Progress:  globalExportState.progress,
		StartedAt: globalExportState.startedAt,
		DoneAt:    globalExportState.doneAt,
		Error:     globalExportState.err,
	}
	globalExportState.mu.Unlock()
	writeJSON(w, http.StatusOK, resp)
}

// runExportSimulation simulates progress for an export or import job.
func runExportSimulation(op string) {
	for i := 1; i <= 10; i++ {
		time.Sleep(100 * time.Millisecond)
		globalExportState.mu.Lock()
		globalExportState.progress = i * 10
		globalExportState.mu.Unlock()
	}
	globalExportState.mu.Lock()
	globalExportState.running = false
	globalExportState.doneAt = time.Now()
	globalExportState.mu.Unlock()
	_ = op
}

func handleGetBackupSchedule(w http.ResponseWriter, _ *http.Request) {
	globalBackupSchedule.mu.RLock()
	defer globalBackupSchedule.mu.RUnlock()
	writeJSON(w, http.StatusOK, globalBackupSchedule)
}

func handlePutBackupSchedule(w http.ResponseWriter, r *http.Request) {
	var req struct {
		CronExpr    string `json:"cron"`
		Destination string `json:"destination"`
		Enabled     bool   `json:"enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON: " + err.Error()})
		return
	}
	globalBackupSchedule.mu.Lock()
	globalBackupSchedule.CronExpr = req.CronExpr
	globalBackupSchedule.Destination = req.Destination
	globalBackupSchedule.Enabled = req.Enabled
	globalBackupSchedule.mu.Unlock()
	writeJSON(w, http.StatusOK, map[string]string{"status": "backup schedule updated"})
}
