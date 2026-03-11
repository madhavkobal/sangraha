package s3

import (
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/madhavkobal/sangraha/internal/storage"
)

// putObject handles PUT /{bucket}/{key} — PutObject and PUT subresources.
func (h *Handler) putObject(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	// Multipart UploadPart is also a PUT with ?partNumber&uploadId.
	if q.Has("uploadId") {
		h.uploadPart(w, r)
		return
	}
	// Phase 2: object tagging.
	if q.Has("tagging") {
		h.putObjectTagging(w, r)
		return
	}
	// CopyObject uses x-amz-copy-source header.
	if r.Header.Get("x-amz-copy-source") != "" {
		h.copyObject(w, r)
		return
	}

	bucket := chi.URLParam(r, "bucket")
	key := chi.URLParam(r, "*")
	identity := identityFromContext(r.Context())

	size := r.ContentLength
	ct := r.Header.Get("Content-Type")

	// SSE algorithm from request header.
	sseAlg := r.Header.Get("x-amz-server-side-encryption")

	// Collect user metadata (x-amz-meta-* headers).
	userMeta := extractUserMeta(r)

	// Tags from x-amz-tagging header.
	tags := extractTagsFromQuery(r)

	out, err := h.engine.PutObject(r.Context(), storage.PutObjectInput{
		Bucket:       bucket,
		Key:          key,
		Body:         r.Body,
		Size:         size,
		ContentType:  ct,
		Owner:        identity.Owner,
		UserMeta:     userMeta,
		Tags:         tags,
		SSEAlgorithm: sseAlg,
	})
	if err != nil {
		switch err.(type) {
		case *storage.BucketNotFoundError:
			writeError(w, r, http.StatusNotFound, "NoSuchBucket", "The specified bucket does not exist")
		default:
			writeError(w, r, http.StatusInternalServerError, "InternalError", err.Error())
		}
		return
	}
	w.Header().Set("ETag", out.ETag)
	if out.VersionID != "" {
		w.Header().Set("x-amz-version-id", out.VersionID)
	}
	if out.SSEAlgorithm != "" {
		w.Header().Set("x-amz-server-side-encryption", out.SSEAlgorithm)
	}
	w.WriteHeader(http.StatusOK)
}

// getObject handles GET /{bucket}/{key} — GetObject, and GET subresources.
func (h *Handler) getObject(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	if q.Has("tagging") {
		h.getObjectTagging(w, r)
		return
	}

	bucket := chi.URLParam(r, "bucket")
	key := chi.URLParam(r, "*")

	out, err := h.engine.GetObject(r.Context(), storage.GetObjectInput{
		Bucket:    bucket,
		Key:       key,
		VersionID: q.Get("versionId"),
	})
	if err != nil {
		if isObjectNotFound(err) {
			writeError(w, r, http.StatusNotFound, "NoSuchKey", "The specified key does not exist")
			return
		}
		if isBucketNotFound(err) {
			writeError(w, r, http.StatusNotFound, "NoSuchBucket", "The specified bucket does not exist")
			return
		}
		writeError(w, r, http.StatusInternalServerError, "InternalError", err.Error())
		return
	}
	defer func() { _ = out.Body.Close() }()

	rec := out.Record
	w.Header().Set("ETag", rec.ETag)
	w.Header().Set("Content-Type", rec.ContentType)
	w.Header().Set("Content-Length", strconv.FormatInt(rec.Size, 10))
	w.Header().Set("Last-Modified", rec.LastModified.UTC().Format(http.TimeFormat))
	for k, v := range rec.UserMeta {
		w.Header().Set("x-amz-meta-"+k, v)
	}
	if rec.VersionID != "" {
		w.Header().Set("x-amz-version-id", rec.VersionID)
	}
	if rec.SSEAlgorithm != "" {
		w.Header().Set("x-amz-server-side-encryption", rec.SSEAlgorithm)
	}
	w.WriteHeader(http.StatusOK)
	_, _ = io.Copy(w, out.Body)
}

// headObject handles HEAD /{bucket}/{key} — HeadObject.
func (h *Handler) headObject(w http.ResponseWriter, r *http.Request) {
	bucket := chi.URLParam(r, "bucket")
	key := chi.URLParam(r, "*")

	rec, err := h.engine.HeadObject(r.Context(), bucket, key)
	if err != nil {
		if isObjectNotFound(err) {
			writeError(w, r, http.StatusNotFound, "NoSuchKey", "The specified key does not exist")
			return
		}
		if isBucketNotFound(err) {
			writeError(w, r, http.StatusNotFound, "NoSuchBucket", "The specified bucket does not exist")
			return
		}
		writeError(w, r, http.StatusInternalServerError, "InternalError", err.Error())
		return
	}
	w.Header().Set("ETag", rec.ETag)
	w.Header().Set("Content-Type", rec.ContentType)
	w.Header().Set("Content-Length", strconv.FormatInt(rec.Size, 10))
	w.Header().Set("Last-Modified", rec.LastModified.UTC().Format(http.TimeFormat))
	for k, v := range rec.UserMeta {
		w.Header().Set("x-amz-meta-"+k, v)
	}
	w.WriteHeader(http.StatusOK)
}

// deleteObject handles DELETE /{bucket}/{key} — DeleteObject (and DELETE subresources).
func (h *Handler) deleteObject(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	// Multipart abort is DELETE with ?uploadId.
	if q.Has("uploadId") {
		h.abortMultipartUpload(w, r)
		return
	}
	// Phase 2: delete object tags.
	if q.Has("tagging") {
		h.deleteObjectTagging(w, r)
		return
	}

	bucket := chi.URLParam(r, "bucket")
	key := chi.URLParam(r, "*")
	identity := identityFromContext(r.Context())

	out, err := h.engine.DeleteObject(r.Context(), storage.DeleteObjectInput{
		Bucket:    bucket,
		Key:       key,
		VersionID: q.Get("versionId"),
		Owner:     identity.Owner,
	})
	if err != nil {
		if isBucketNotFound(err) {
			writeError(w, r, http.StatusNotFound, "NoSuchBucket", "The specified bucket does not exist")
			return
		}
		writeError(w, r, http.StatusInternalServerError, "InternalError", err.Error())
		return
	}
	if out.VersionID != "" {
		w.Header().Set("x-amz-version-id", out.VersionID)
	}
	if out.DeleteMarker {
		w.Header().Set("x-amz-delete-marker", "true")
	}
	w.WriteHeader(http.StatusNoContent)
}

// copyObject handles PUT /{bucket}/{key} with x-amz-copy-source — CopyObject.
func (h *Handler) copyObject(w http.ResponseWriter, r *http.Request) {
	dstBucket := chi.URLParam(r, "bucket")
	dstKey := chi.URLParam(r, "*")
	identity := identityFromContext(r.Context())

	// x-amz-copy-source is /<srcBucket>/<srcKey> (URL-encoded).
	copySource := r.Header.Get("x-amz-copy-source")
	copySource = strings.TrimPrefix(copySource, "/")
	parts := strings.SplitN(copySource, "/", 2)
	if len(parts) != 2 {
		writeError(w, r, http.StatusBadRequest, "InvalidArgument", "x-amz-copy-source must be /bucket/key")
		return
	}
	srcBucket, srcKey := parts[0], parts[1]

	rec, err := h.engine.CopyObject(r.Context(), srcBucket, srcKey, dstBucket, dstKey, identity.Owner)
	if err != nil {
		switch err.(type) {
		case *storage.ObjectNotFoundError:
			writeError(w, r, http.StatusNotFound, "NoSuchKey", "The specified key does not exist")
		case *storage.BucketNotFoundError:
			writeError(w, r, http.StatusNotFound, "NoSuchBucket", "The specified bucket does not exist")
		default:
			writeError(w, r, http.StatusInternalServerError, "InternalError", err.Error())
		}
		return
	}

	writeXML(w, r, http.StatusOK, struct {
		XMLName      struct{} `xml:"CopyObjectResult"`
		LastModified string   `xml:"LastModified"`
		ETag         string   `xml:"ETag"`
	}{
		LastModified: rec.LastModified.UTC().Format("2006-01-02T15:04:05.000Z"),
		ETag:         rec.ETag,
	})
}

// postObject dispatches POST /{bucket}/{key} to multipart operations.
func (h *Handler) postObject(w http.ResponseWriter, r *http.Request) {
	if r.URL.Query().Has("uploads") {
		h.createMultipartUpload(w, r)
		return
	}
	if r.URL.Query().Has("uploadId") {
		h.completeMultipartUpload(w, r)
		return
	}
	writeError(w, r, http.StatusBadRequest, "InvalidRequest", "unsupported object POST operation")
}

// extractUserMeta collects x-amz-meta-* headers into a map.
func extractUserMeta(r *http.Request) map[string]string {
	meta := map[string]string{}
	prefix := "x-amz-meta-"
	for k, vals := range r.Header {
		lower := strings.ToLower(k)
		if strings.HasPrefix(lower, prefix) {
			meta[strings.TrimPrefix(lower, prefix)] = strings.Join(vals, ",")
		}
	}
	return meta
}

func isObjectNotFound(err error) bool {
	_, ok := err.(*storage.ObjectNotFoundError)
	return ok
}

// parseContentRange parses a Range header like "bytes=0-1023".
// Returns offset and length (-1 if unset).
func parseContentRange(rangeHeader string) (offset, length int64, err error) {
	if rangeHeader == "" {
		return 0, -1, nil
	}
	rangeHeader = strings.TrimPrefix(rangeHeader, "bytes=")
	parts := strings.SplitN(rangeHeader, "-", 2)
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("invalid Range header: %s", rangeHeader)
	}
	if parts[0] == "" {
		return 0, -1, nil
	}
	offset, err = strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return 0, 0, fmt.Errorf("invalid Range start: %w", err)
	}
	if parts[1] != "" {
		end, err2 := strconv.ParseInt(parts[1], 10, 64)
		if err2 != nil {
			return 0, 0, fmt.Errorf("invalid Range end: %w", err2)
		}
		length = end - offset + 1
	} else {
		length = -1
	}
	return offset, length, nil
}
