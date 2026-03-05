package admin

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/madhavkobal/sangraha/internal/auth"
)

// presignHandler generates presigned S3 URLs.
type presignHandler struct {
	keyStore  *auth.KeyStore
	serverURL string
}

type presignRequest struct {
	Bucket    string `json:"bucket"`
	Key       string `json:"key"`
	Method    string `json:"method"` // "GET" | "PUT"
	ExpiresIn int    `json:"expires_in"` // seconds, max 604800 (7 days)
	Region    string `json:"region"`
}

type presignResponse struct {
	URL       string    `json:"url"`
	ExpiresAt time.Time `json:"expires_at"`
}

func (h *presignHandler) create(w http.ResponseWriter, r *http.Request) {
	var req presignRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}
	if req.Bucket == "" || req.Key == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "bucket and key are required"})
		return
	}
	if req.Method == "" {
		req.Method = "GET"
	}
	if req.ExpiresIn <= 0 || req.ExpiresIn > 604800 {
		req.ExpiresIn = 3600 // default 1 hour
	}
	if req.Region == "" {
		req.Region = "us-east-1"
	}

	// Get the caller's access key from context.
	identity, ok := identityFromContext(r)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "not authenticated"})
		return
	}

	rec, err := h.keyStore.Lookup(r.Context(), identity.AccessKey)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "key lookup failed"})
		return
	}

	now := time.Now().UTC()
	expires := time.Duration(req.ExpiresIn) * time.Second

	url, err := auth.GeneratePresignedURL(
		req.Method,
		h.serverURL,
		req.Bucket,
		req.Key,
		rec.AccessKey,
		rec.SigningKey,
		req.Region,
		expires,
		now,
	)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, presignResponse{
		URL:       url,
		ExpiresAt: now.Add(expires),
	})
}
