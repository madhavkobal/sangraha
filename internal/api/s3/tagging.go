package s3

import (
	"encoding/xml"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/madhavkobal/sangraha/internal/metadata"
)

// xmlTagSet is the XML representation of a tag set.
type xmlTagSet struct {
	XMLName xml.Name `xml:"Tagging"`
	TagSet  []xmlTag `xml:"TagSet>Tag"`
}

type xmlTag struct {
	Key   string `xml:"Key"`
	Value string `xml:"Value"`
}

func tagsFromXML(ts xmlTagSet) map[string]string {
	if len(ts.TagSet) == 0 {
		return nil
	}
	m := make(map[string]string, len(ts.TagSet))
	for _, t := range ts.TagSet {
		m[t.Key] = t.Value
	}
	return m
}

func tagsToXML(tags map[string]string) xmlTagSet {
	ts := xmlTagSet{}
	for k, v := range tags {
		ts.TagSet = append(ts.TagSet, xmlTag{Key: k, Value: v})
	}
	return ts
}

// getObjectTagging handles GET /{bucket}/{key}?tagging.
func (h *Handler) getObjectTagging(w http.ResponseWriter, r *http.Request) {
	bucket := chi.URLParam(r, "bucket")
	key := chi.URLParam(r, "*")
	tags, err := h.engine.GetObjectTags(r.Context(), bucket, key)
	if err != nil {
		if isObjectNotFound(err) {
			writeError(w, r, http.StatusNotFound, "NoSuchKey", "The specified key does not exist")
			return
		}
		writeError(w, r, http.StatusInternalServerError, "InternalError", err.Error())
		return
	}
	writeXML(w, r, http.StatusOK, tagsToXML(tags))
}

// putObjectTagging handles PUT /{bucket}/{key}?tagging.
func (h *Handler) putObjectTagging(w http.ResponseWriter, r *http.Request) {
	bucket := chi.URLParam(r, "bucket")
	key := chi.URLParam(r, "*")
	var ts xmlTagSet
	if err := xml.NewDecoder(r.Body).Decode(&ts); err != nil {
		writeError(w, r, http.StatusBadRequest, "MalformedXML", "invalid tagging XML")
		return
	}
	if err := h.engine.SetObjectTags(r.Context(), bucket, key, tagsFromXML(ts)); err != nil {
		if isObjectNotFound(err) {
			writeError(w, r, http.StatusNotFound, "NoSuchKey", "The specified key does not exist")
			return
		}
		writeError(w, r, http.StatusInternalServerError, "InternalError", err.Error())
		return
	}
	w.WriteHeader(http.StatusOK)
}

// deleteObjectTagging handles DELETE /{bucket}/{key}?tagging.
func (h *Handler) deleteObjectTagging(w http.ResponseWriter, r *http.Request) {
	bucket := chi.URLParam(r, "bucket")
	key := chi.URLParam(r, "*")
	if err := h.engine.DeleteObjectTags(r.Context(), bucket, key); err != nil {
		if isObjectNotFound(err) {
			writeError(w, r, http.StatusNotFound, "NoSuchKey", "The specified key does not exist")
			return
		}
		writeError(w, r, http.StatusInternalServerError, "InternalError", err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// getBucketTagging handles GET /{bucket}?tagging.
func (h *Handler) getBucketTagging(w http.ResponseWriter, r *http.Request, bucket string) {
	tags, err := h.engine.GetBucketTags(r.Context(), bucket)
	if err != nil {
		if isBucketNotFound(err) {
			writeError(w, r, http.StatusNotFound, "NoSuchBucket", "The specified bucket does not exist")
			return
		}
		writeError(w, r, http.StatusInternalServerError, "InternalError", err.Error())
		return
	}
	writeXML(w, r, http.StatusOK, tagsToXML(tags))
}

// putBucketTagging handles PUT /{bucket}?tagging.
func (h *Handler) putBucketTagging(w http.ResponseWriter, r *http.Request, bucket string) {
	var ts xmlTagSet
	if err := xml.NewDecoder(r.Body).Decode(&ts); err != nil {
		writeError(w, r, http.StatusBadRequest, "MalformedXML", "invalid tagging XML")
		return
	}
	if err := h.engine.SetBucketTags(r.Context(), bucket, tagsFromXML(ts)); err != nil {
		if isBucketNotFound(err) {
			writeError(w, r, http.StatusNotFound, "NoSuchBucket", "The specified bucket does not exist")
			return
		}
		writeError(w, r, http.StatusInternalServerError, "InternalError", err.Error())
		return
	}
	w.WriteHeader(http.StatusOK)
}

// xmlBucketTagging returns the bucket tags wrapped with the proper type alias.
func xmlBucketTagging(tags map[string]string) xmlTagSet {
	return tagsToXML(tags)
}

// --- Helpers for tag extraction ---

// extractTagsFromQuery parses x-amz-tagging header or query param.
func extractTagsFromQuery(r *http.Request) map[string]string {
	tagging := r.Header.Get("x-amz-tagging")
	if tagging == "" {
		return nil
	}
	tags := map[string]string{}
	for _, pair := range splitTagging(tagging) {
		if kv := splitTagKV(pair); len(kv) == 2 {
			tags[kv[0]] = kv[1]
		}
	}
	return tags
}

func splitTagging(s string) []string {
	var parts []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '&' {
			parts = append(parts, s[start:i])
			start = i + 1
		}
	}
	parts = append(parts, s[start:])
	return parts
}

func splitTagKV(s string) []string {
	for i := 0; i < len(s); i++ {
		if s[i] == '=' {
			return []string{s[:i], s[i+1:]}
		}
	}
	return []string{s}
}

// xmlMetadataTags converts a metadata tag map (for internal use).
func xmlMetadataTags(tags map[string]string) []xmlTag {
	_ = metadata.ObjectRecord{}
	out := make([]xmlTag, 0, len(tags))
	for k, v := range tags {
		out = append(out, xmlTag{Key: k, Value: v})
	}
	return out
}
