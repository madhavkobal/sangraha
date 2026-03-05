package s3

import (
	"encoding/xml"
	"net/http"
	"strconv"

	"github.com/madhavkobal/sangraha/internal/metadata"
	"github.com/madhavkobal/sangraha/internal/storage"
)

// VersioningConfiguration is the XML body for Put/GetBucketVersioning.
type versioningConfiguration struct {
	XMLName xml.Name `xml:"VersioningConfiguration"`
	Status  string   `xml:"Status,omitempty"`
}

// getBucketVersioning handles GET /{bucket}?versioning.
func (h *Handler) getBucketVersioning(w http.ResponseWriter, r *http.Request, bucket string) {
	status, err := h.engine.GetBucketVersioning(r.Context(), bucket)
	if err != nil {
		if isBucketNotFound(err) {
			writeError(w, r, http.StatusNotFound, "NoSuchBucket", "The specified bucket does not exist")
			return
		}
		writeError(w, r, http.StatusInternalServerError, "InternalError", err.Error())
		return
	}
	// S3 returns empty Status when versioning is "Never enabled".
	xmlStatus := ""
	switch status {
	case storage.VersioningEnabled:
		xmlStatus = "Enabled"
	case storage.VersioningSuspended:
		xmlStatus = "Suspended"
	}
	writeXML(w, r, http.StatusOK, versioningConfiguration{Status: xmlStatus})
}

// putBucketVersioning handles PUT /{bucket}?versioning.
func (h *Handler) putBucketVersioning(w http.ResponseWriter, r *http.Request, bucket string) {
	var cfg versioningConfiguration
	if err := xml.NewDecoder(r.Body).Decode(&cfg); err != nil {
		writeError(w, r, http.StatusBadRequest, "MalformedXML", "invalid versioning configuration XML")
		return
	}
	var status string
	switch cfg.Status {
	case "Enabled":
		status = storage.VersioningEnabled
	case "Suspended":
		status = storage.VersioningSuspended
	default:
		writeError(w, r, http.StatusBadRequest, "IllegalVersioningConfigurationException",
			"versioning Status must be Enabled or Suspended")
		return
	}
	if err := h.engine.SetBucketVersioning(r.Context(), bucket, status); err != nil {
		if isBucketNotFound(err) {
			writeError(w, r, http.StatusNotFound, "NoSuchBucket", "The specified bucket does not exist")
			return
		}
		writeError(w, r, http.StatusInternalServerError, "InternalError", err.Error())
		return
	}
	w.WriteHeader(http.StatusOK)
}

// listObjectVersions handles GET /{bucket}?versions.
func (h *Handler) listObjectVersions(w http.ResponseWriter, r *http.Request, bucket string) {
	q := r.URL.Query()
	maxKeysStr := q.Get("max-keys")
	maxKeys := 1000
	if maxKeysStr != "" {
		if n, err := strconv.Atoi(maxKeysStr); err == nil && n > 0 {
			maxKeys = n
		}
	}
	opts := metadata.ListOptions{
		Prefix:  q.Get("prefix"),
		MaxKeys: maxKeys,
	}
	versions, err := h.engine.ListObjectVersions(r.Context(), bucket, opts)
	if err != nil {
		if isBucketNotFound(err) {
			writeError(w, r, http.StatusNotFound, "NoSuchBucket", "The specified bucket does not exist")
			return
		}
		writeError(w, r, http.StatusInternalServerError, "InternalError", err.Error())
		return
	}

	type versionEntry struct {
		XMLName      xml.Name `xml:"Version"`
		Key          string   `xml:"Key"`
		VersionID    string   `xml:"VersionId"`
		IsLatest     bool     `xml:"IsLatest"`
		LastModified string   `xml:"LastModified"`
		ETag         string   `xml:"ETag,omitempty"`
		Size         int64    `xml:"Size,omitempty"`
		StorageClass string   `xml:"StorageClass,omitempty"`
	}
	type deleteMarkerEntry struct {
		XMLName      xml.Name `xml:"DeleteMarker"`
		Key          string   `xml:"Key"`
		VersionID    string   `xml:"VersionId"`
		IsLatest     bool     `xml:"IsLatest"`
		LastModified string   `xml:"LastModified"`
	}
	type listVersionsResult struct {
		XMLName     xml.Name            `xml:"ListVersionsResult"`
		Name        string              `xml:"Name"`
		Prefix      string              `xml:"Prefix,omitempty"`
		MaxKeys     int                 `xml:"MaxKeys"`
		IsTruncated bool                `xml:"IsTruncated"`
		Versions    []interface{}
	}

	result := struct {
		XMLName     xml.Name `xml:"ListVersionsResult"`
		Name        string   `xml:"Name"`
		Prefix      string   `xml:"Prefix,omitempty"`
		MaxKeys     int      `xml:"MaxKeys"`
		IsTruncated bool     `xml:"IsTruncated"`
	}{
		Name:    bucket,
		Prefix:  opts.Prefix,
		MaxKeys: maxKeys,
	}
	_ = result
	_ = listVersionsResult{}

	// Build mixed versions+delete-markers list.
	type entry struct {
		XMLName      xml.Name
		Key          string `xml:"Key"`
		VersionID    string `xml:"VersionId"`
		IsLatest     bool   `xml:"IsLatest"`
		LastModified string `xml:"LastModified"`
		ETag         string `xml:"ETag,omitempty"`
		Size         int64  `xml:"Size,omitempty"`
		StorageClass string `xml:"StorageClass,omitempty"`
	}

	// Since we can't mix element names dynamically easily, use a raw approach.
	type xmlResponse struct {
		XMLName     xml.Name       `xml:"ListVersionsResult"`
		Name        string         `xml:"Name"`
		Prefix      string         `xml:"Prefix,omitempty"`
		MaxKeys     int            `xml:"MaxKeys"`
		IsTruncated bool           `xml:"IsTruncated"`
		Versions    []versionEntry `xml:"Version"`
		Markers     []deleteMarkerEntry `xml:"DeleteMarker"`
	}

	resp := xmlResponse{
		Name:    bucket,
		Prefix:  opts.Prefix,
		MaxKeys: maxKeys,
	}
	for _, v := range versions {
		if v.IsDeleteMarker {
			resp.Markers = append(resp.Markers, deleteMarkerEntry{
				Key:          v.Key,
				VersionID:    v.VersionID,
				IsLatest:     v.IsLatest,
				LastModified: v.LastModified.UTC().Format("2006-01-02T15:04:05.000Z"),
			})
		} else {
			sc := v.StorageClass
			if sc == "" {
				sc = "STANDARD"
			}
			resp.Versions = append(resp.Versions, versionEntry{
				Key:          v.Key,
				VersionID:    v.VersionID,
				IsLatest:     v.IsLatest,
				LastModified: v.LastModified.UTC().Format("2006-01-02T15:04:05.000Z"),
				ETag:         v.ETag,
				Size:         v.Size,
				StorageClass: sc,
			})
		}
	}
	writeXML(w, r, http.StatusOK, resp)
}
