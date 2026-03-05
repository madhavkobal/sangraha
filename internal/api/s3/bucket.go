package s3

import (
	"encoding/xml"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/madhavkobal/sangraha/internal/storage"
	"github.com/madhavkobal/sangraha/pkg/s3types"
)

// listBuckets handles GET / — ListBuckets.
func (h *Handler) listBuckets(w http.ResponseWriter, r *http.Request) {
	identity := identityFromContext(r.Context())

	buckets, err := h.engine.ListBuckets(r.Context())
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "InternalError", err.Error())
		return
	}

	result := s3types.ListAllMyBucketsResult{
		Owner: s3types.Owner{ID: identity.AccessKey, DisplayName: identity.Owner},
	}
	for _, b := range buckets {
		result.Buckets = append(result.Buckets, s3types.Bucket{
			Name:         b.Name,
			CreationDate: b.CreatedAt,
		})
	}
	writeXML(w, r, http.StatusOK, result)
}

// createBucket handles PUT /{bucket} — CreateBucket and PUT subresource dispatch.
func (h *Handler) createBucket(w http.ResponseWriter, r *http.Request) {
	bucket := chi.URLParam(r, "bucket")
	q := r.URL.Query()

	// Phase 2: dispatch PUT subresources.
	switch {
	case q.Has("versioning"):
		h.putBucketVersioning(w, r, bucket)
		return
	case q.Has("cors"):
		h.putBucketCORS(w, r, bucket)
		return
	case q.Has("policy"):
		h.putBucketPolicy(w, r, bucket)
		return
	case q.Has("lifecycle"):
		h.putBucketLifecycle(w, r, bucket)
		return
	case q.Has("tagging"):
		h.putBucketTagging(w, r, bucket)
		return
	case q.Has("encryption"):
		h.putBucketEncryption(w, r, bucket)
		return
	case q.Has("acl"):
		h.putBucketACL(w, r, bucket)
		return
	}

	identity := identityFromContext(r.Context())

	var region string
	if r.ContentLength > 0 {
		var cfg s3types.CreateBucketConfiguration
		dec := xml.NewDecoder(r.Body)
		if err := dec.Decode(&cfg); err == nil {
			region = cfg.LocationConstraint
		}
	}

	if err := h.engine.CreateBucket(r.Context(), bucket, identity.Owner, region); err != nil {
		switch err.(type) {
		case *storage.BucketAlreadyExistsError:
			writeError(w, r, http.StatusConflict, "BucketAlreadyOwnedByYou", err.Error())
		default:
			writeError(w, r, http.StatusBadRequest, "InvalidBucketName", err.Error())
		}
		return
	}
	w.Header().Set("Location", "/"+bucket)
	w.WriteHeader(http.StatusOK)
}

// headBucket handles HEAD /{bucket} — HeadBucket.
func (h *Handler) headBucket(w http.ResponseWriter, r *http.Request) {
	bucket := chi.URLParam(r, "bucket")

	if _, err := h.engine.HeadBucket(r.Context(), bucket); err != nil {
		if isBucketNotFound(err) {
			writeError(w, r, http.StatusNotFound, "NoSuchBucket", "The specified bucket does not exist")
			return
		}
		writeError(w, r, http.StatusInternalServerError, "InternalError", err.Error())
		return
	}
	w.WriteHeader(http.StatusOK)
}

// deleteBucket handles DELETE /{bucket} — DeleteBucket and DELETE subresources.
func (h *Handler) deleteBucket(w http.ResponseWriter, r *http.Request) {
	bucket := chi.URLParam(r, "bucket")
	q := r.URL.Query()

	// Phase 2: dispatch DELETE subresources.
	switch {
	case q.Has("cors"):
		if err := h.engine.DeleteCORSRules(r.Context(), bucket); err != nil {
			writeError(w, r, http.StatusInternalServerError, "InternalError", err.Error())
			return
		}
		w.WriteHeader(http.StatusNoContent)
		return
	case q.Has("policy"):
		if err := h.engine.DeleteBucketPolicy(r.Context(), bucket); err != nil {
			writeError(w, r, http.StatusInternalServerError, "InternalError", err.Error())
			return
		}
		w.WriteHeader(http.StatusNoContent)
		return
	case q.Has("lifecycle"):
		if err := h.engine.DeleteLifecycleRules(r.Context(), bucket); err != nil {
			writeError(w, r, http.StatusInternalServerError, "InternalError", err.Error())
			return
		}
		w.WriteHeader(http.StatusNoContent)
		return
	case q.Has("tagging"):
		if err := h.engine.DeleteBucketTags(r.Context(), bucket); err != nil {
			writeError(w, r, http.StatusInternalServerError, "InternalError", err.Error())
			return
		}
		w.WriteHeader(http.StatusNoContent)
		return
	case q.Has("encryption"):
		if err := h.engine.SetBucketEncryption(r.Context(), bucket, ""); err != nil {
			writeError(w, r, http.StatusInternalServerError, "InternalError", err.Error())
			return
		}
		w.WriteHeader(http.StatusNoContent)
		return
	}

	if err := h.engine.DeleteBucket(r.Context(), bucket); err != nil {
		switch err.(type) {
		case *storage.BucketNotFoundError:
			writeError(w, r, http.StatusNotFound, "NoSuchBucket", "The specified bucket does not exist")
		case *storage.BucketNotEmptyError:
			writeError(w, r, http.StatusConflict, "BucketNotEmpty", "The bucket you tried to delete is not empty")
		default:
			writeError(w, r, http.StatusInternalServerError, "InternalError", err.Error())
		}
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// postBucket handles POST /{bucket}?delete — DeleteObjects.
func (h *Handler) postBucket(w http.ResponseWriter, r *http.Request) {
	bucket := chi.URLParam(r, "bucket")
	if r.URL.Query().Has("delete") {
		h.deleteObjects(w, r, bucket)
		return
	}
	writeError(w, r, http.StatusBadRequest, "InvalidRequest", "unsupported bucket POST operation")
}

// headBucket also dispatches Phase 2 GET subresource queries via the listObjects path,
// but the main dispatch for subresources is in listObjects and the new PUT handler.
// Additional Phase 2 bucket operations are dispatched here.
func (h *Handler) getBucketSubresource(w http.ResponseWriter, r *http.Request) {
	bucket := chi.URLParam(r, "bucket")
	q := r.URL.Query()
	switch {
	case q.Has("versioning"):
		h.getBucketVersioning(w, r, bucket)
	case q.Has("cors"):
		h.getBucketCORS(w, r, bucket)
	case q.Has("policy"):
		h.getBucketPolicy(w, r, bucket)
	case q.Has("lifecycle"):
		h.getBucketLifecycle(w, r, bucket)
	case q.Has("tagging"):
		h.getBucketTagging(w, r, bucket)
	case q.Has("encryption"):
		h.getBucketEncryption(w, r, bucket)
	case q.Has("acl"):
		h.getBucketACL(w, r, bucket)
	case q.Has("versions"):
		h.listObjectVersions(w, r, bucket)
	default:
		writeError(w, r, http.StatusBadRequest, "InvalidRequest", "unsupported subresource")
	}
}

func (h *Handler) putBucketSubresource(w http.ResponseWriter, r *http.Request) {
	bucket := chi.URLParam(r, "bucket")
	q := r.URL.Query()
	switch {
	case q.Has("versioning"):
		h.putBucketVersioning(w, r, bucket)
	case q.Has("cors"):
		h.putBucketCORS(w, r, bucket)
	case q.Has("policy"):
		h.putBucketPolicy(w, r, bucket)
	case q.Has("lifecycle"):
		h.putBucketLifecycle(w, r, bucket)
	case q.Has("tagging"):
		h.putBucketTagging(w, r, bucket)
	case q.Has("encryption"):
		h.putBucketEncryption(w, r, bucket)
	case q.Has("acl"):
		h.putBucketACL(w, r, bucket)
	default:
		// Fall through to create-bucket for plain PUT /{bucket}
		h.createBucket(w, r)
	}
}

func (h *Handler) deleteBucketSubresource(w http.ResponseWriter, r *http.Request) {
	bucket := chi.URLParam(r, "bucket")
	q := r.URL.Query()
	switch {
	case q.Has("cors"):
		if err := h.engine.DeleteCORSRules(r.Context(), bucket); err != nil {
			writeError(w, r, http.StatusInternalServerError, "InternalError", err.Error())
			return
		}
		w.WriteHeader(http.StatusNoContent)
	case q.Has("policy"):
		if err := h.engine.DeleteBucketPolicy(r.Context(), bucket); err != nil {
			writeError(w, r, http.StatusInternalServerError, "InternalError", err.Error())
			return
		}
		w.WriteHeader(http.StatusNoContent)
	case q.Has("lifecycle"):
		if err := h.engine.DeleteLifecycleRules(r.Context(), bucket); err != nil {
			writeError(w, r, http.StatusInternalServerError, "InternalError", err.Error())
			return
		}
		w.WriteHeader(http.StatusNoContent)
	case q.Has("tagging"):
		if err := h.engine.DeleteBucketTags(r.Context(), bucket); err != nil {
			writeError(w, r, http.StatusInternalServerError, "InternalError", err.Error())
			return
		}
		w.WriteHeader(http.StatusNoContent)
	case q.Has("encryption"):
		if err := h.engine.SetBucketEncryption(r.Context(), bucket, ""); err != nil {
			writeError(w, r, http.StatusInternalServerError, "InternalError", err.Error())
			return
		}
		w.WriteHeader(http.StatusNoContent)
	default:
		h.deleteBucket(w, r)
	}
}

// deleteObjects handles POST /{bucket}?delete — DeleteObjects.
func (h *Handler) deleteObjects(w http.ResponseWriter, r *http.Request, bucket string) {
	var req s3types.DeleteObjectsRequest
	if err := xml.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, r, http.StatusBadRequest, "MalformedXML", "invalid request XML")
		return
	}

	keys := make([]string, len(req.Objects))
	for i, obj := range req.Objects {
		keys[i] = obj.Key
	}

	identity := identityFromContext(r.Context())
	deleted, errs := h.engine.DeleteObjects(r.Context(), bucket, keys, identity.Owner)

	result := s3types.DeleteResult{}
	for _, key := range deleted {
		result.Deleted = append(result.Deleted, s3types.Deleted{Key: key})
	}
	for key, err := range errs {
		result.Errors = append(result.Errors, s3types.DeleteError{
			Key:     key,
			Code:    "InternalError",
			Message: err.Error(),
		})
	}
	writeXML(w, r, http.StatusOK, result)
}

func isBucketNotFound(err error) bool {
	_, ok := err.(*storage.BucketNotFoundError)
	return ok
}
