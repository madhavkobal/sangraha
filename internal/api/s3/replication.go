package s3

import (
	"encoding/xml"
	"net/http"

	"github.com/madhavkobal/sangraha/internal/metadata"
)

// putBucketReplication handles PUT /{bucket}?replication.
func (h *Handler) putBucketReplication(w http.ResponseWriter, r *http.Request, bucket string) {
	var req xmlReplicationConfiguration
	if err := xml.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, r, http.StatusBadRequest, "MalformedXML", "invalid replication configuration XML")
		return
	}

	cfg := &metadata.ReplicationConfig{Role: req.Role}
	for _, xr := range req.Rules {
		rule := metadata.ReplicationRule{
			ID:     xr.ID,
			Status: xr.Status,
			Filter: metadata.LifecycleFilter{Prefix: xr.Filter.Prefix},
			Destination: metadata.ReplicationDest{
				BucketARN:    xr.Destination.Bucket,
				StorageClass: xr.Destination.StorageClass,
			},
			DeleteMarkerReplication: xr.DeleteMarkerReplication.Status,
		}
		cfg.Rules = append(cfg.Rules, rule)
	}

	if len(cfg.Rules) == 0 {
		cfg = nil
	}

	if err := h.engine.SetBucketReplication(r.Context(), bucket, cfg); err != nil {
		if isBucketNotFound(err) {
			writeError(w, r, http.StatusNotFound, "NoSuchBucket", "The specified bucket does not exist")
			return
		}
		writeError(w, r, http.StatusInternalServerError, "InternalError", err.Error())
		return
	}
	w.WriteHeader(http.StatusOK)
}

// getBucketReplication handles GET /{bucket}?replication.
func (h *Handler) getBucketReplication(w http.ResponseWriter, r *http.Request, bucket string) {
	cfg, err := h.engine.GetBucketReplication(r.Context(), bucket)
	if err != nil {
		if isBucketNotFound(err) {
			writeError(w, r, http.StatusNotFound, "NoSuchBucket", "The specified bucket does not exist")
			return
		}
		writeError(w, r, http.StatusInternalServerError, "InternalError", err.Error())
		return
	}
	if cfg == nil {
		writeError(w, r, http.StatusNotFound, "ReplicationConfigurationNotFoundError",
			"The replication configuration was not found")
		return
	}
	resp := xmlReplicationConfiguration{Role: cfg.Role}
	for _, rule := range cfg.Rules {
		xr := xmlReplicationRule{
			ID:     rule.ID,
			Status: rule.Status,
			Filter: xmlReplFilter{Prefix: rule.Filter.Prefix},
			Destination: xmlReplDest{
				Bucket:       rule.Destination.BucketARN,
				StorageClass: rule.Destination.StorageClass,
			},
		}
		if rule.DeleteMarkerReplication != "" {
			xr.DeleteMarkerReplication = xmlDeleteMarkerReplication{Status: rule.DeleteMarkerReplication}
		}
		resp.Rules = append(resp.Rules, xr)
	}
	writeXML(w, r, http.StatusOK, resp)
}

// deleteBucketReplication handles DELETE /{bucket}?replication.
func (h *Handler) deleteBucketReplication(w http.ResponseWriter, r *http.Request, bucket string) {
	if err := h.engine.SetBucketReplication(r.Context(), bucket, nil); err != nil {
		if isBucketNotFound(err) {
			writeError(w, r, http.StatusNotFound, "NoSuchBucket", "The specified bucket does not exist")
			return
		}
		writeError(w, r, http.StatusInternalServerError, "InternalError", err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// --- XML types ---

type xmlReplicationConfiguration struct {
	XMLName xml.Name              `xml:"ReplicationConfiguration"`
	Role    string                `xml:"Role"`
	Rules   []xmlReplicationRule  `xml:"Rule"`
}

type xmlReplicationRule struct {
	ID                      string                      `xml:"ID,omitempty"`
	Status                  string                      `xml:"Status"`
	Filter                  xmlReplFilter               `xml:"Filter"`
	Destination             xmlReplDest                 `xml:"Destination"`
	DeleteMarkerReplication xmlDeleteMarkerReplication  `xml:"DeleteMarkerReplication,omitempty"`
}

type xmlReplFilter struct {
	Prefix string `xml:"Prefix,omitempty"`
}

type xmlReplDest struct {
	Bucket       string `xml:"Bucket"`
	StorageClass string `xml:"StorageClass,omitempty"`
}

type xmlDeleteMarkerReplication struct {
	Status string `xml:"Status"`
}
