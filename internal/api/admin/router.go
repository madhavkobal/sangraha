package admin

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"

	"github.com/madhavkobal/sangraha/internal/api/middleware"
	"github.com/madhavkobal/sangraha/internal/auth"
)

// New creates the admin API HTTP handler and registers all routes.
func New(keyStore *auth.KeyStore, version, buildTime string) http.Handler {
	uh := &userHandler{keyStore: keyStore}

	r := chi.NewRouter()
	r.Use(chimw.Recoverer)
	r.Use(middleware.RequestID)

	// Health and readiness endpoints (no auth required — used by load balancers).
	r.Get("/admin/v1/health", handleHealth)
	r.Get("/admin/v1/ready", handleReady)
	r.Get("/admin/v1/info", handleInfo(version, buildTime))

	// Prometheus metrics (no auth for simplicity in Phase 1).
	r.Get("/admin/v1/metrics", metricsHandler().ServeHTTP)

	// Admin endpoints require auth.
	r.Group(func(r chi.Router) {
		r.Use(middleware.Auth(keyStore))

		// User management.
		r.Post("/admin/v1/users", uh.create)
		r.Get("/admin/v1/users", uh.list)
		r.Delete("/admin/v1/users/{accessKey}", uh.delete)
		r.Post("/admin/v1/users/{accessKey}/keys/rotate", uh.rotateKey)
	})

	return r
}
