package s3

import (
	"encoding/xml"
	"net/http"

	"github.com/madhavkobal/sangraha/internal/metadata"
)

// xmlCORSConfiguration is the XML body for Put/GetBucketCors.
type xmlCORSConfiguration struct {
	XMLName   xml.Name      `xml:"CORSConfiguration"`
	CORSRules []xmlCORSRule `xml:"CORSRule"`
}

type xmlCORSRule struct {
	ID            string   `xml:"ID,omitempty"`
	AllowedOrigin []string `xml:"AllowedOrigin"`
	AllowedMethod []string `xml:"AllowedMethod"`
	AllowedHeader []string `xml:"AllowedHeader,omitempty"`
	ExposeHeader  []string `xml:"ExposeHeader,omitempty"`
	MaxAgeSeconds int      `xml:"MaxAgeSeconds,omitempty"`
}

func xmlCORSToMeta(cfg xmlCORSConfiguration) []metadata.CORSRule {
	rules := make([]metadata.CORSRule, len(cfg.CORSRules))
	for i, r := range cfg.CORSRules {
		rules[i] = metadata.CORSRule{
			ID:             r.ID,
			AllowedOrigins: r.AllowedOrigin,
			AllowedMethods: r.AllowedMethod,
			AllowedHeaders: r.AllowedHeader,
			ExposeHeaders:  r.ExposeHeader,
			MaxAgeSeconds:  r.MaxAgeSeconds,
		}
	}
	return rules
}

func metaCORSToXML(rules []metadata.CORSRule) xmlCORSConfiguration {
	cfg := xmlCORSConfiguration{}
	for _, r := range rules {
		cfg.CORSRules = append(cfg.CORSRules, xmlCORSRule{
			ID:            r.ID,
			AllowedOrigin: r.AllowedOrigins,
			AllowedMethod: r.AllowedMethods,
			AllowedHeader: r.AllowedHeaders,
			ExposeHeader:  r.ExposeHeaders,
			MaxAgeSeconds: r.MaxAgeSeconds,
		})
	}
	return cfg
}

// getBucketCORS handles GET /{bucket}?cors.
func (h *Handler) getBucketCORS(w http.ResponseWriter, r *http.Request, bucket string) {
	rules, err := h.engine.GetCORSRules(r.Context(), bucket)
	if err != nil {
		if isBucketNotFound(err) {
			writeError(w, r, http.StatusNotFound, "NoSuchBucket", "The specified bucket does not exist")
			return
		}
		writeError(w, r, http.StatusInternalServerError, "InternalError", err.Error())
		return
	}
	if len(rules) == 0 {
		writeError(w, r, http.StatusNotFound, "NoSuchCORSConfiguration", "The CORS configuration does not exist")
		return
	}
	writeXML(w, r, http.StatusOK, metaCORSToXML(rules))
}

// putBucketCORS handles PUT /{bucket}?cors.
func (h *Handler) putBucketCORS(w http.ResponseWriter, r *http.Request, bucket string) {
	var cfg xmlCORSConfiguration
	if err := xml.NewDecoder(r.Body).Decode(&cfg); err != nil {
		writeError(w, r, http.StatusBadRequest, "MalformedXML", "invalid CORS configuration XML")
		return
	}
	rules := xmlCORSToMeta(cfg)
	if err := h.engine.SetCORSRules(r.Context(), bucket, rules); err != nil {
		if isBucketNotFound(err) {
			writeError(w, r, http.StatusNotFound, "NoSuchBucket", "The specified bucket does not exist")
			return
		}
		writeError(w, r, http.StatusInternalServerError, "InternalError", err.Error())
		return
	}
	w.WriteHeader(http.StatusOK)
}
