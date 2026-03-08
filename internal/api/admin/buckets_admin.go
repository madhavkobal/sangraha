package admin

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/madhavkobal/sangraha/internal/metadata"
	"github.com/madhavkobal/sangraha/internal/storage"
)

// bucketAdminHandler provides admin-level bucket and object management endpoints.
type bucketAdminHandler struct {
	engine *storage.Engine
}

// bucketAdminRecord is the JSON representation of a bucket for the admin API.
type bucketAdminRecord struct {
	Name         string    `json:"name"`
	Owner        string    `json:"owner"`
	Region       string    `json:"region"`
	Versioning   string    `json:"versioning"`
	ACL          string    `json:"acl"`
	ObjectCount  int64     `json:"object_count"`
	TotalBytes   int64     `json:"total_bytes"`
	CreatedAt    time.Time `json:"created_at"`
	SSEAlgorithm string    `json:"sse_algorithm,omitempty"`
}

func bucketToRecord(b metadata.BucketRecord) bucketAdminRecord {
	return bucketAdminRecord{
		Name:         b.Name,
		Owner:        b.Owner,
		Region:       b.Region,
		Versioning:   b.Versioning,
		ACL:          b.ACL,
		ObjectCount:  b.ObjectCount,
		TotalBytes:   b.TotalBytes,
		CreatedAt:    b.CreatedAt,
		SSEAlgorithm: b.SSEAlgorithm,
	}
}

// listBuckets handles GET /admin/v1/buckets.
func (h *bucketAdminHandler) listBuckets(w http.ResponseWriter, r *http.Request) {
	buckets, err := h.engine.ListBuckets(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	out := make([]bucketAdminRecord, len(buckets))
	for i, b := range buckets {
		out[i] = bucketToRecord(b)
	}
	writeJSON(w, http.StatusOK, out)
}

// createBucketRequest is the JSON body for POST /admin/v1/buckets.
type createBucketRequest struct {
	Name   string `json:"name"`
	Region string `json:"region"`
	ACL    string `json:"acl"`
}

// createBucket handles POST /admin/v1/buckets.
func (h *bucketAdminHandler) createBucket(w http.ResponseWriter, r *http.Request) {
	var req createBucketRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}
	if req.Name == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "name is required"})
		return
	}
	id, ok := identityFromContext(r)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	if err := h.engine.CreateBucket(r.Context(), req.Name, id.AccessKey, req.Region); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	b, err := h.engine.HeadBucket(r.Context(), req.Name)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusCreated, bucketToRecord(b))
}

// deleteBucket handles DELETE /admin/v1/buckets/{name}.
func (h *bucketAdminHandler) deleteBucket(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	if err := h.engine.DeleteBucket(r.Context(), name); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// objectAdminRecord is the JSON representation of an object for the admin API.
type objectAdminRecord struct {
	Key          string            `json:"key"`
	Size         int64             `json:"size"`
	ETag         string            `json:"etag"`
	ContentType  string            `json:"content_type"`
	LastModified time.Time         `json:"last_modified"`
	Owner        string            `json:"owner"`
	StorageClass string            `json:"storage_class"`
	Tags         map[string]string `json:"tags,omitempty"`
	VersionID    string            `json:"version_id,omitempty"`
}

// listObjects handles GET /admin/v1/buckets/{name}/objects.
func (h *bucketAdminHandler) listObjects(w http.ResponseWriter, r *http.Request) {
	bucket := chi.URLParam(r, "name")
	q := r.URL.Query()
	prefix := q.Get("prefix")
	maxKeysStr := q.Get("max_keys")
	maxKeys := 100
	if maxKeysStr != "" {
		if n, err := strconv.Atoi(maxKeysStr); err == nil && n > 0 && n <= 1000 {
			maxKeys = n
		}
	}
	objs, prefixes, err := h.engine.ListObjects(r.Context(), bucket, metadata.ListOptions{
		Prefix:            prefix,
		Delimiter:         q.Get("delimiter"),
		ContinuationToken: q.Get("continuation_token"),
		MaxKeys:           maxKeys,
	})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	out := make([]objectAdminRecord, len(objs))
	for i, o := range objs {
		out[i] = objectAdminRecord{
			Key:          o.Key,
			Size:         o.Size,
			ETag:         o.ETag,
			ContentType:  o.ContentType,
			LastModified: o.LastModified,
			Owner:        o.Owner,
			StorageClass: o.StorageClass,
			Tags:         o.Tags,
			VersionID:    o.VersionID,
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"objects":  out,
		"prefixes": prefixes,
	})
}

// deleteObject handles DELETE /admin/v1/buckets/{name}/objects/{key}.
func (h *bucketAdminHandler) deleteObject(w http.ResponseWriter, r *http.Request) {
	bucket := chi.URLParam(r, "name")
	key := chi.URLParam(r, "*")
	id, _ := identityFromContext(r)
	_, err := h.engine.DeleteObject(r.Context(), storage.DeleteObjectInput{
		Bucket: bucket,
		Key:    key,
		Owner:  id.AccessKey,
	})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
