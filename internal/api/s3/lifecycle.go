package s3

import (
	"encoding/xml"
	"net/http"
	"time"

	"github.com/madhavkobal/sangraha/internal/metadata"
)

// xmlLifecycleConfiguration is the XML body for Put/GetBucketLifecycle.
type xmlLifecycleConfiguration struct {
	XMLName xml.Name           `xml:"LifecycleConfiguration"`
	Rules   []xmlLifecycleRule `xml:"Rule"`
}

type xmlLifecycleRule struct {
	ID                             string               `xml:"ID,omitempty"`
	Status                         string               `xml:"Status"`
	Filter                         xmlLifecycleFilter   `xml:"Filter,omitempty"`
	Expiration                     *xmlExpiration       `xml:"Expiration,omitempty"`
	AbortIncompleteMultipartUpload *xmlAbortIncomplete  `xml:"AbortIncompleteMultipartUpload,omitempty"`
	NoncurrentVersionExpiration    *xmlNoncurrentExpiry `xml:"NoncurrentVersionExpiration,omitempty"`
}

type xmlLifecycleFilter struct {
	Prefix string   `xml:"Prefix,omitempty"`
	Tag    []xmlTag `xml:"Tag,omitempty"`
}

type xmlExpiration struct {
	Days int    `xml:"Days,omitempty"`
	Date string `xml:"Date,omitempty"`
}

type xmlAbortIncomplete struct {
	DaysAfterInitiation int `xml:"DaysAfterInitiation"`
}

type xmlNoncurrentExpiry struct {
	NoncurrentDays int `xml:"NoncurrentDays"`
}

func xmlLifecycleToMeta(cfg xmlLifecycleConfiguration) []metadata.LifecycleRule {
	rules := make([]metadata.LifecycleRule, 0, len(cfg.Rules))
	for _, r := range cfg.Rules {
		rule := metadata.LifecycleRule{
			ID:     r.ID,
			Status: r.Status,
			Filter: metadata.LifecycleFilter{Prefix: r.Filter.Prefix},
		}
		if len(r.Filter.Tag) > 0 {
			rule.Filter.Tags = make(map[string]string)
			for _, t := range r.Filter.Tag {
				rule.Filter.Tags[t.Key] = t.Value
			}
		}
		if r.Expiration != nil {
			rule.ExpirationDays = r.Expiration.Days
			if r.Expiration.Date != "" {
				if t, err := time.Parse("2006-01-02", r.Expiration.Date); err == nil {
					rule.ExpirationDate = &t
				}
			}
		}
		if r.AbortIncompleteMultipartUpload != nil {
			rule.AbortIncompleteMultipartDays = r.AbortIncompleteMultipartUpload.DaysAfterInitiation
		}
		if r.NoncurrentVersionExpiration != nil {
			rule.NoncurrentVersionExpirationDays = r.NoncurrentVersionExpiration.NoncurrentDays
		}
		rules = append(rules, rule)
	}
	return rules
}

func metaLifecycleToXML(rules []metadata.LifecycleRule) xmlLifecycleConfiguration {
	cfg := xmlLifecycleConfiguration{}
	for _, r := range rules {
		xr := xmlLifecycleRule{
			ID:     r.ID,
			Status: r.Status,
			Filter: xmlLifecycleFilter{Prefix: r.Filter.Prefix},
		}
		for k, v := range r.Filter.Tags {
			xr.Filter.Tag = append(xr.Filter.Tag, xmlTag{Key: k, Value: v})
		}
		if r.ExpirationDays > 0 || r.ExpirationDate != nil {
			xr.Expiration = &xmlExpiration{Days: r.ExpirationDays}
			if r.ExpirationDate != nil {
				xr.Expiration.Date = r.ExpirationDate.Format("2006-01-02")
			}
		}
		if r.AbortIncompleteMultipartDays > 0 {
			xr.AbortIncompleteMultipartUpload = &xmlAbortIncomplete{
				DaysAfterInitiation: r.AbortIncompleteMultipartDays,
			}
		}
		if r.NoncurrentVersionExpirationDays > 0 {
			xr.NoncurrentVersionExpiration = &xmlNoncurrentExpiry{
				NoncurrentDays: r.NoncurrentVersionExpirationDays,
			}
		}
		cfg.Rules = append(cfg.Rules, xr)
	}
	return cfg
}

// getBucketLifecycle handles GET /{bucket}?lifecycle.
func (h *Handler) getBucketLifecycle(w http.ResponseWriter, r *http.Request, bucket string) {
	rules, err := h.engine.GetLifecycleRules(r.Context(), bucket)
	if err != nil {
		if isBucketNotFound(err) {
			writeError(w, r, http.StatusNotFound, "NoSuchBucket", "The specified bucket does not exist")
			return
		}
		writeError(w, r, http.StatusInternalServerError, "InternalError", err.Error())
		return
	}
	if len(rules) == 0 {
		writeError(w, r, http.StatusNotFound, "NoSuchLifecycleConfiguration",
			"The lifecycle configuration does not exist")
		return
	}
	writeXML(w, r, http.StatusOK, metaLifecycleToXML(rules))
}

// putBucketLifecycle handles PUT /{bucket}?lifecycle.
func (h *Handler) putBucketLifecycle(w http.ResponseWriter, r *http.Request, bucket string) {
	var cfg xmlLifecycleConfiguration
	if err := xml.NewDecoder(r.Body).Decode(&cfg); err != nil {
		writeError(w, r, http.StatusBadRequest, "MalformedXML", "invalid lifecycle configuration XML")
		return
	}
	rules := xmlLifecycleToMeta(cfg)
	if err := h.engine.SetLifecycleRules(r.Context(), bucket, rules); err != nil {
		if isBucketNotFound(err) {
			writeError(w, r, http.StatusNotFound, "NoSuchBucket", "The specified bucket does not exist")
			return
		}
		writeError(w, r, http.StatusInternalServerError, "InternalError", err.Error())
		return
	}
	w.WriteHeader(http.StatusOK)
}

// getBucketEncryption handles GET /{bucket}?encryption.
func (h *Handler) getBucketEncryption(w http.ResponseWriter, r *http.Request, bucket string) {
	alg, err := h.engine.GetBucketEncryption(r.Context(), bucket)
	if err != nil {
		if isBucketNotFound(err) {
			writeError(w, r, http.StatusNotFound, "NoSuchBucket", "The specified bucket does not exist")
			return
		}
		writeError(w, r, http.StatusInternalServerError, "InternalError", err.Error())
		return
	}
	if alg == "" {
		writeError(w, r, http.StatusNotFound, "ServerSideEncryptionConfigurationNotFoundError",
			"The server side encryption configuration was not found")
		return
	}
	type rule struct {
		ApplyServerSideEncryptionByDefault struct {
			SSEAlgorithm string `xml:"SSEAlgorithm"`
		} `xml:"ApplyServerSideEncryptionByDefault"`
	}
	type encConfig struct {
		XMLName xml.Name `xml:"ServerSideEncryptionConfiguration"`
		Rules   []rule   `xml:"Rule"`
	}
	cfg := encConfig{}
	var r2 rule
	r2.ApplyServerSideEncryptionByDefault.SSEAlgorithm = alg
	cfg.Rules = append(cfg.Rules, r2)
	writeXML(w, r, http.StatusOK, cfg)
}

// putBucketEncryption handles PUT /{bucket}?encryption.
func (h *Handler) putBucketEncryption(w http.ResponseWriter, r *http.Request, bucket string) {
	type rule struct {
		ApplyServerSideEncryptionByDefault struct {
			SSEAlgorithm string `xml:"SSEAlgorithm"`
		} `xml:"ApplyServerSideEncryptionByDefault"`
	}
	type encConfig struct {
		XMLName xml.Name `xml:"ServerSideEncryptionConfiguration"`
		Rules   []rule   `xml:"Rule"`
	}
	var cfg encConfig
	if err := xml.NewDecoder(r.Body).Decode(&cfg); err != nil {
		writeError(w, r, http.StatusBadRequest, "MalformedXML", "invalid encryption configuration XML")
		return
	}
	alg := ""
	if len(cfg.Rules) > 0 {
		alg = cfg.Rules[0].ApplyServerSideEncryptionByDefault.SSEAlgorithm
	}
	if err := h.engine.SetBucketEncryption(r.Context(), bucket, alg); err != nil {
		if isBucketNotFound(err) {
			writeError(w, r, http.StatusNotFound, "NoSuchBucket", "The specified bucket does not exist")
			return
		}
		writeError(w, r, http.StatusBadRequest, "InvalidArgument", err.Error())
		return
	}
	w.WriteHeader(http.StatusOK)
}
