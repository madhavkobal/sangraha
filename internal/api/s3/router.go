package s3

import (
	"context"
	"net/http"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"

	"github.com/madhavkobal/sangraha/internal/api/middleware"
	"github.com/madhavkobal/sangraha/internal/audit"
	"github.com/madhavkobal/sangraha/internal/auth"
	"github.com/madhavkobal/sangraha/internal/metadata"
	"github.com/madhavkobal/sangraha/internal/storage"
)

// Handler is the root HTTP handler for the S3-compatible API.
type Handler struct {
	engine   *storage.Engine
	keyStore *auth.KeyStore
	auditor  *audit.Logger
}

// New creates an S3 API handler and registers all routes on a new chi router.
func New(engine *storage.Engine, keyStore *auth.KeyStore, auditor *audit.Logger, rateLimitRPS int) http.Handler {
	h := &Handler{
		engine:   engine,
		keyStore: keyStore,
		auditor:  auditor,
	}

	r := chi.NewRouter()
	r.Use(chimw.Recoverer)
	r.Use(middleware.RequestID)
	r.Use(middleware.Audit(auditor))

	// CORS middleware: fetch rules from the storage engine.
	corsRulesFetcher := func(bucket string) []metadata.CORSRule {
		rules, err := engine.GetCORSRules(context.Background(), bucket)
		if err != nil {
			return nil
		}
		return rules
	}
	r.Use(middleware.CORS(corsRulesFetcher))

	// Authentication: all S3 routes require SigV4.
	r.Use(middleware.Auth(keyStore))

	// Rate limiting (after auth so we can limit per access key too).
	r.Use(middleware.RateLimit(rateLimitRPS))

	// Service-level operations.
	r.Get("/", h.listBuckets)

	// Bucket and object operations.
	r.Route("/{bucket}", func(r chi.Router) {
		// Bucket-level operations.
		r.Put("/", h.createBucket)
		r.Head("/", h.headBucket)
		r.Delete("/", h.deleteBucket)
		r.Get("/", h.listObjects)
		r.Post("/", h.postBucket)

		// Object operations: keys may contain '/' so use /* wildcard.
		r.Put("/*", h.putObject)
		r.Get("/*", h.getObject)
		r.Head("/*", h.headObject)
		r.Delete("/*", h.deleteObject)
		r.Post("/*", h.postObject)
	})

	return r
}
