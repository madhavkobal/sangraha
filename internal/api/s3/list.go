package s3

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/madhavkobal/sangraha/internal/metadata"
	"github.com/madhavkobal/sangraha/pkg/s3types"
)

const defaultMaxKeys = 1000

// listObjects handles GET /{bucket} — dispatches based on query parameters.
// list-type=2 → ListObjectsV2; versions → ListObjectVersions (not yet
// implemented, returns V1 format); otherwise → legacy ListObjects (V1).
func (h *Handler) listObjects(w http.ResponseWriter, r *http.Request) {
	bucket := chi.URLParam(r, "bucket")
	q := r.URL.Query()

	if q.Get("list-type") == "2" {
		h.listObjectsV2(w, r, bucket)
		return
	}
	// Legacy ListObjects V1 (also used as fallback for ?versions in Phase 1).
	h.listObjectsV1(w, r, bucket)
}

// listObjectsV2 implements ListObjectsV2 (list-type=2 or default).
func (h *Handler) listObjectsV2(w http.ResponseWriter, r *http.Request, bucket string) {
	q := r.URL.Query()

	maxKeys := defaultMaxKeys
	if s := q.Get("max-keys"); s != "" {
		n, err := strconv.Atoi(s)
		if err != nil || n < 0 {
			writeError(w, r, http.StatusBadRequest, "InvalidArgument", "invalid max-keys")
			return
		}
		if n < maxKeys {
			maxKeys = n
		}
	}

	opts := metadata.ListOptions{
		Prefix:            q.Get("prefix"),
		Delimiter:         q.Get("delimiter"),
		ContinuationToken: q.Get("continuation-token"),
		StartAfter:        q.Get("start-after"),
		MaxKeys:           maxKeys,
	}

	records, commonPrefixes, err := h.engine.ListObjects(r.Context(), bucket, opts)
	if err != nil {
		if isBucketNotFound(err) {
			writeError(w, r, http.StatusNotFound, "NoSuchBucket", "The specified bucket does not exist")
			return
		}
		writeError(w, r, http.StatusInternalServerError, "InternalError", err.Error())
		return
	}

	result := s3types.ListBucketResult{
		Name:              bucket,
		Prefix:            opts.Prefix,
		Delimiter:         opts.Delimiter,
		MaxKeys:           maxKeys,
		ContinuationToken: opts.ContinuationToken,
		StartAfter:        opts.StartAfter,
		KeyCount:          len(records),
		IsTruncated:       len(records) == maxKeys,
	}
	if result.IsTruncated && len(records) > 0 {
		result.NextContinuationToken = records[len(records)-1].Key
	}
	for _, rec := range records {
		result.Contents = append(result.Contents, s3types.Object{
			Key:          rec.Key,
			LastModified: rec.LastModified,
			ETag:         rec.ETag,
			Size:         rec.Size,
			StorageClass: rec.StorageClass,
		})
	}
	for _, cp := range commonPrefixes {
		result.CommonPrefixes = append(result.CommonPrefixes, s3types.CommonPrefix{Prefix: cp})
	}
	writeXML(w, r, http.StatusOK, result)
}

// listObjectsV1 implements the legacy ListObjects (V1) format.
func (h *Handler) listObjectsV1(w http.ResponseWriter, r *http.Request, bucket string) {
	// V1 uses 'marker' instead of continuation-token but logic is similar.
	q := r.URL.Query()
	opts := metadata.ListOptions{
		Prefix:     q.Get("prefix"),
		Delimiter:  q.Get("delimiter"),
		StartAfter: q.Get("marker"),
		MaxKeys:    defaultMaxKeys,
	}
	if s := q.Get("max-keys"); s != "" {
		if n, err := strconv.Atoi(s); err == nil && n >= 0 && n < defaultMaxKeys {
			opts.MaxKeys = n
		}
	}

	records, commonPrefixes, err := h.engine.ListObjects(r.Context(), bucket, opts)
	if err != nil {
		if isBucketNotFound(err) {
			writeError(w, r, http.StatusNotFound, "NoSuchBucket", "The specified bucket does not exist")
			return
		}
		writeError(w, r, http.StatusInternalServerError, "InternalError", err.Error())
		return
	}

	result := s3types.ListBucketResult{
		Name:        bucket,
		Prefix:      opts.Prefix,
		Delimiter:   opts.Delimiter,
		MaxKeys:     opts.MaxKeys,
		KeyCount:    len(records),
		IsTruncated: len(records) == opts.MaxKeys,
	}
	for _, rec := range records {
		result.Contents = append(result.Contents, s3types.Object{
			Key:          rec.Key,
			LastModified: rec.LastModified,
			ETag:         rec.ETag,
			Size:         rec.Size,
			StorageClass: rec.StorageClass,
		})
	}
	for _, cp := range commonPrefixes {
		result.CommonPrefixes = append(result.CommonPrefixes, s3types.CommonPrefix{Prefix: cp})
	}
	writeXML(w, r, http.StatusOK, result)
}
