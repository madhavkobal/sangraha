package s3

import (
	"io"
	"net/http"
)

// getBucketPolicy handles GET /{bucket}?policy.
func (h *Handler) getBucketPolicy(w http.ResponseWriter, r *http.Request, bucket string) {
	policy, err := h.engine.GetBucketPolicy(r.Context(), bucket)
	if err != nil {
		if isBucketNotFound(err) {
			writeError(w, r, http.StatusNotFound, "NoSuchBucket", "The specified bucket does not exist")
			return
		}
		writeError(w, r, http.StatusInternalServerError, "InternalError", err.Error())
		return
	}
	if policy == "" {
		writeError(w, r, http.StatusNotFound, "NoSuchBucketPolicy", "The bucket policy does not exist")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(policy)) //nolint:gosec // G705: policy is operator-stored JSON; Content-Type is application/json, no HTML rendering
}

// putBucketPolicy handles PUT /{bucket}?policy.
func (h *Handler) putBucketPolicy(w http.ResponseWriter, r *http.Request, bucket string) {
	body, err := io.ReadAll(io.LimitReader(r.Body, 20*1024)) // 20 KB max
	if err != nil {
		writeError(w, r, http.StatusBadRequest, "InvalidArgument", "could not read policy body")
		return
	}
	if err := h.engine.SetBucketPolicy(r.Context(), bucket, string(body)); err != nil {
		if isBucketNotFound(err) {
			writeError(w, r, http.StatusNotFound, "NoSuchBucket", "The specified bucket does not exist")
			return
		}
		writeError(w, r, http.StatusBadRequest, "InvalidArgument", err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// getBucketACL handles GET /{bucket}?acl.
func (h *Handler) getBucketACL(w http.ResponseWriter, r *http.Request, bucket string) {
	rec, err := h.engine.HeadBucket(r.Context(), bucket)
	if err != nil {
		if isBucketNotFound(err) {
			writeError(w, r, http.StatusNotFound, "NoSuchBucket", "The specified bucket does not exist")
			return
		}
		writeError(w, r, http.StatusInternalServerError, "InternalError", err.Error())
		return
	}
	identity := identityFromContext(r.Context())
	type grant struct {
		Grantee    interface{} `xml:"Grantee"`
		Permission string      `xml:"Permission"`
	}
	type aclResponse struct {
		Owner struct {
			ID          string `xml:"ID"`
			DisplayName string `xml:"DisplayName"`
		} `xml:"Owner"`
		AccessControlList []grant `xml:"AccessControlList>Grant"`
	}
	resp := aclResponse{}
	resp.Owner.ID = rec.Owner
	resp.Owner.DisplayName = identity.Owner
	writeXML(w, r, http.StatusOK, resp)
}

// putBucketACL handles PUT /{bucket}?acl.
func (h *Handler) putBucketACL(w http.ResponseWriter, r *http.Request, bucket string) {
	acl := r.Header.Get("x-amz-acl")
	if acl == "" {
		acl = "private"
	}
	if err := h.engine.SetBucketACL(r.Context(), bucket, acl); err != nil {
		if isBucketNotFound(err) {
			writeError(w, r, http.StatusNotFound, "NoSuchBucket", "The specified bucket does not exist")
			return
		}
		writeError(w, r, http.StatusBadRequest, "InvalidArgument", err.Error())
		return
	}
	w.WriteHeader(http.StatusOK)
}
