package s3

import (
	"encoding/xml"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/madhavkobal/sangraha/internal/storage"
	"github.com/madhavkobal/sangraha/pkg/s3types"
)

// createMultipartUpload handles POST /{bucket}/{key}?uploads.
func (h *Handler) createMultipartUpload(w http.ResponseWriter, r *http.Request) {
	bucket := chi.URLParam(r, "bucket")
	key := chi.URLParam(r, "*")
	identity := identityFromContext(r.Context())

	ct := r.Header.Get("Content-Type")
	userMeta := extractUserMeta(r)

	uploadID, err := h.engine.CreateMultipartUpload(r.Context(), bucket, key, identity.Owner, ct, userMeta)
	if err != nil {
		if isBucketNotFound(err) {
			writeError(w, r, http.StatusNotFound, "NoSuchBucket", "The specified bucket does not exist")
			return
		}
		writeError(w, r, http.StatusInternalServerError, "InternalError", err.Error())
		return
	}
	writeXML(w, r, http.StatusOK, s3types.InitiateMultipartUploadResult{
		Bucket:   bucket,
		Key:      key,
		UploadID: uploadID,
	})
}

// uploadPart handles PUT /{bucket}/{key}?partNumber=N&uploadId=ID.
func (h *Handler) uploadPart(w http.ResponseWriter, r *http.Request) {
	bucket := chi.URLParam(r, "bucket")
	key := chi.URLParam(r, "*")
	_ = bucket
	_ = key

	q := r.URL.Query()
	uploadID := q.Get("uploadId")
	if uploadID == "" {
		writeError(w, r, http.StatusBadRequest, "InvalidArgument", "uploadId is required")
		return
	}
	partNumberStr := q.Get("partNumber")
	partNumber, err := strconv.Atoi(partNumberStr)
	if err != nil || partNumber < 1 || partNumber > 10000 {
		writeError(w, r, http.StatusBadRequest, "InvalidArgument", "partNumber must be between 1 and 10000")
		return
	}

	etag, err := h.engine.UploadPart(r.Context(), storage.UploadPartInput{
		UploadID:   uploadID,
		PartNumber: partNumber,
		Body:       r.Body,
		Size:       r.ContentLength,
	})
	if err != nil {
		if isMultipartNotFound(err) {
			writeError(w, r, http.StatusNotFound, "NoSuchUpload", "The specified upload does not exist")
			return
		}
		writeError(w, r, http.StatusInternalServerError, "InternalError", err.Error())
		return
	}
	w.Header().Set("ETag", etag)
	w.WriteHeader(http.StatusOK)
}

// completeMultipartUpload handles POST /{bucket}/{key}?uploadId=ID.
func (h *Handler) completeMultipartUpload(w http.ResponseWriter, r *http.Request) {
	bucket := chi.URLParam(r, "bucket")
	key := chi.URLParam(r, "*")
	uploadID := r.URL.Query().Get("uploadId")
	if uploadID == "" {
		writeError(w, r, http.StatusBadRequest, "InvalidArgument", "uploadId is required")
		return
	}

	var req s3types.CompleteMultipartUpload
	if err := xml.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, r, http.StatusBadRequest, "MalformedXML", "invalid request XML")
		return
	}

	parts := make([]storage.CompletePart, len(req.Parts))
	for i, p := range req.Parts {
		parts[i] = storage.CompletePart{PartNumber: p.PartNumber, ETag: p.ETag}
	}

	rec, err := h.engine.CompleteMultipartUpload(r.Context(), storage.CompleteMultipartInput{
		UploadID: uploadID,
		Parts:    parts,
	})
	if err != nil {
		if isMultipartNotFound(err) {
			writeError(w, r, http.StatusNotFound, "NoSuchUpload", "The specified upload does not exist")
			return
		}
		writeError(w, r, http.StatusInternalServerError, "InternalError", err.Error())
		return
	}

	scheme := "https"
	if r.TLS == nil {
		scheme = "http"
	}
	location := scheme + "://" + r.Host + "/" + bucket + "/" + key

	writeXML(w, r, http.StatusOK, s3types.CompleteMultipartUploadResult{
		Location: location,
		Bucket:   bucket,
		Key:      key,
		ETag:     rec.ETag,
	})
}

// abortMultipartUpload handles DELETE /{bucket}/{key}?uploadId=ID.
func (h *Handler) abortMultipartUpload(w http.ResponseWriter, r *http.Request) {
	uploadID := r.URL.Query().Get("uploadId")
	if uploadID == "" {
		writeError(w, r, http.StatusBadRequest, "InvalidArgument", "uploadId is required")
		return
	}
	if err := h.engine.AbortMultipartUpload(r.Context(), uploadID); err != nil {
		if isMultipartNotFound(err) {
			writeError(w, r, http.StatusNotFound, "NoSuchUpload", "The specified upload does not exist")
			return
		}
		writeError(w, r, http.StatusInternalServerError, "InternalError", err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func isMultipartNotFound(err error) bool {
	_, ok := err.(*storage.MultipartNotFoundError)
	return ok
}
