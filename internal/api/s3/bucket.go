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

// createBucket handles PUT /{bucket} — CreateBucket.
func (h *Handler) createBucket(w http.ResponseWriter, r *http.Request) {
	bucket := chi.URLParam(r, "bucket")
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

// deleteBucket handles DELETE /{bucket} — DeleteBucket.
func (h *Handler) deleteBucket(w http.ResponseWriter, r *http.Request) {
	bucket := chi.URLParam(r, "bucket")

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

	deleted, errs := h.engine.DeleteObjects(r.Context(), bucket, keys)

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
