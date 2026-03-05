package s3

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"

	"github.com/madhavkobal/sangraha/internal/api/middleware"
	"github.com/madhavkobal/sangraha/internal/audit"
	"github.com/madhavkobal/sangraha/internal/auth"
	"github.com/madhavkobal/sangraha/internal/storage"
)

// Handler is the root HTTP handler for the S3-compatible API.
type Handler struct {
	engine   *storage.Engine
	keyStore *auth.KeyStore
	auditor  *audit.Logger
}

// New creates an S3 API handler and registers all routes on a new chi router.
func New(engine *storage.Engine, keyStore *auth.KeyStore, auditor *audit.Logger) http.Handler {
	h := &Handler{
		engine:   engine,
		keyStore: keyStore,
		auditor:  auditor,
	}

	r := chi.NewRouter()
	r.Use(chimw.Recoverer)
	r.Use(middleware.RequestID)
	r.Use(middleware.Audit(auditor))

	// Authentication: all S3 routes require SigV4.
	r.Use(middleware.Auth(keyStore))

	// Service-level operations.
	r.Get("/", h.listBuckets)

	// Bucket and object operations are disambiguated by subresource query params.
	r.Route("/{bucket}", func(r chi.Router) {
		// Bucket operations (no object key in path).
		r.Put("/", h.createBucket)
		r.Head("/", h.headBucket)
		r.Delete("/", h.deleteBucket)
		r.Get("/", h.listObjects)
		r.Post("/", h.postBucket) // batch delete

		// Object operations: keys may contain '/' so use /* wildcard.
		// Handlers extract the key via chi.URLParam(r, "*").
		r.Put("/*", h.putObject)
		r.Get("/*", h.getObject)
		r.Head("/*", h.headObject)
		r.Delete("/*", h.deleteObject)
		r.Post("/*", h.postObject) // multipart initiate or complete
	})

	return r
}
