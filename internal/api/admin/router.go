package admin

import (
	"net/http"
	"sync"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"

	"github.com/madhavkobal/sangraha/internal/api/middleware"
	"github.com/madhavkobal/sangraha/internal/audit"
	"github.com/madhavkobal/sangraha/internal/auth"
	"github.com/madhavkobal/sangraha/internal/config"
	"github.com/madhavkobal/sangraha/internal/storage"
	"github.com/madhavkobal/sangraha/internal/web"
)

// New creates the admin API HTTP handler and registers all routes.
// cfg is the current running configuration (mutations via PUT /admin/v1/config
// apply in-place and are protected by a mutex inside configHandler).
func New(
	keyStore *auth.KeyStore,
	engine *storage.Engine,
	auditor *audit.Logger,
	version, buildTime, serverURL string,
	cfg *config.Config,
) http.Handler {
	uh := &userHandler{keyStore: keyStore}
	ph := &presignHandler{keyStore: keyStore, serverURL: serverURL}
	ch := &configHandler{cfg: cfg, mu: sync.RWMutex{}}
	bh := &bucketAdminHandler{engine: engine}
	ah := &alertHandler{}
	auh := &auditHandler{auditLogPath: cfg.Logging.AuditLog}

	r := chi.NewRouter()
	r.Use(chimw.Recoverer)
	r.Use(middleware.RequestID)

	// Health and readiness endpoints (no auth required — used by load balancers).
	r.Get("/admin/v1/health", handleHealth)
	r.Get("/admin/v1/ready", handleReady)
	r.Get("/admin/v1/info", handleInfo(version, buildTime))

	// Prometheus metrics (no auth for simplicity).
	r.Get("/admin/v1/metrics", metricsHandler().ServeHTTP)

	// Log stream SSE (no auth — same as metrics, operator-facing only).
	r.Get("/admin/v1/logs/stream", handleLogStream)

	// Admin endpoints require auth.
	r.Group(func(r chi.Router) {
		r.Use(middleware.Auth(keyStore))

		// User management.
		r.Post("/admin/v1/users", uh.create)
		r.Get("/admin/v1/users", uh.list)
		r.Delete("/admin/v1/users/{accessKey}", uh.delete)
		r.Post("/admin/v1/users/{accessKey}/keys/rotate", uh.rotateKey)

		// Presigned URL generation.
		r.Post("/admin/v1/presign", ph.create)

		// Configuration management.
		r.Get("/admin/v1/config", ch.get)
		r.Put("/admin/v1/config", ch.update)
		r.Post("/admin/v1/config/validate", ch.validate)

		// TLS management.
		r.Get("/admin/v1/tls", handleTLSInfo)
		r.Post("/admin/v1/tls/renew", handleTLSRenew)

		// Server control.
		r.Post("/admin/v1/server/reload", handleServerReload)
		r.Get("/admin/v1/connections", handleConnections)

		// Garbage collection.
		r.Post("/admin/v1/gc", handleGCTrigger)
		r.Get("/admin/v1/gc/status", handleGCStatus)

		// Bucket + object management (admin view).
		r.Get("/admin/v1/buckets", bh.listBuckets)
		r.Post("/admin/v1/buckets", bh.createBucket)
		r.Delete("/admin/v1/buckets/{name}", bh.deleteBucket)
		r.Get("/admin/v1/buckets/{name}/objects", bh.listObjects)
		r.Delete("/admin/v1/buckets/{name}/objects/*", bh.deleteObject)

		// Quota management (Phase 3).
		r.Get("/admin/v1/buckets/{name}/quota", bh.getQuota)
		r.Put("/admin/v1/buckets/{name}/quota", bh.putQuota)
		r.Delete("/admin/v1/buckets/{name}/quota", bh.deleteQuota)

		// Alert rules + history.
		r.Get("/admin/v1/alerts", ah.listRules)
		r.Post("/admin/v1/alerts", ah.createRule)
		r.Delete("/admin/v1/alerts/{id}", ah.deleteRule)
		r.Get("/admin/v1/alerts/history", ah.listHistory)

		// Audit log query.
		r.Get("/admin/v1/audit", auh.handleAuditQuery)

		// Export / import.
		r.Post("/admin/v1/export", handleExport)
		r.Post("/admin/v1/import", handleImport)
		r.Get("/admin/v1/export/status", handleExportStatus)

		// Backup schedule.
		r.Get("/admin/v1/backup/schedule", handleGetBackupSchedule)
		r.Put("/admin/v1/backup/schedule", handlePutBackupSchedule)
	})

	// Serve embedded web dashboard at root; must be last so API routes take priority.
	r.Mount("/", web.Handler())

	return r
}

// identityFromContext retrieves the authenticated identity from the context.
func identityFromContext(r *http.Request) (auth.VerifiedIdentity, bool) {
	return middleware.IdentityFromContext(r.Context())
}
