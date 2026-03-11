package s3

import (
	"encoding/xml"
	"net/http"

	"github.com/madhavkobal/sangraha/internal/metadata"
)

// putBucketWebsite handles PUT /{bucket}?website.
func (h *Handler) putBucketWebsite(w http.ResponseWriter, r *http.Request, bucket string) {
	var req xmlWebsiteConfiguration
	if err := xml.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, r, http.StatusBadRequest, "MalformedXML", "invalid website configuration XML")
		return
	}
	cfg := &metadata.WebsiteConfig{
		IndexDocument: req.IndexDocument.Suffix,
		ErrorDocument: req.ErrorDocument.Key,
	}
	for _, rule := range req.RoutingRules {
		cfg.RoutingRules = append(cfg.RoutingRules, metadata.WebsiteRoutingRule{
			Condition: metadata.WebsiteCondition{
				KeyPrefixEquals:             rule.Condition.KeyPrefixEquals,
				HTTPErrorCodeReturnedEquals: rule.Condition.HTTPErrorCodeReturnedEquals,
			},
			Redirect: metadata.WebsiteRedirect{
				HostName:             rule.Redirect.HostName,
				Protocol:             rule.Redirect.Protocol,
				ReplaceKeyPrefixWith: rule.Redirect.ReplaceKeyPrefixWith,
				ReplaceKeyWith:       rule.Redirect.ReplaceKeyWith,
				HTTPRedirectCode:     rule.Redirect.HTTPRedirectCode,
			},
		})
	}
	if err := h.engine.SetBucketWebsite(r.Context(), bucket, cfg); err != nil {
		if isBucketNotFound(err) {
			writeError(w, r, http.StatusNotFound, "NoSuchBucket", "The specified bucket does not exist")
			return
		}
		writeError(w, r, http.StatusInternalServerError, "InternalError", err.Error())
		return
	}
	w.WriteHeader(http.StatusOK)
}

// getBucketWebsite handles GET /{bucket}?website.
func (h *Handler) getBucketWebsite(w http.ResponseWriter, r *http.Request, bucket string) {
	cfg, err := h.engine.GetBucketWebsite(r.Context(), bucket)
	if err != nil {
		if isBucketNotFound(err) {
			writeError(w, r, http.StatusNotFound, "NoSuchBucket", "The specified bucket does not exist")
			return
		}
		writeError(w, r, http.StatusInternalServerError, "InternalError", err.Error())
		return
	}
	if cfg == nil {
		writeError(w, r, http.StatusNotFound, "NoSuchWebsiteConfiguration",
			"The specified bucket does not have a website configuration")
		return
	}
	resp := xmlWebsiteConfiguration{
		IndexDocument: xmlIndexDocument{Suffix: cfg.IndexDocument},
		ErrorDocument: xmlErrorDocument{Key: cfg.ErrorDocument},
	}
	for _, rule := range cfg.RoutingRules {
		resp.RoutingRules = append(resp.RoutingRules, xmlRoutingRule{
			Condition: xmlRoutingCondition{
				KeyPrefixEquals:             rule.Condition.KeyPrefixEquals,
				HTTPErrorCodeReturnedEquals: rule.Condition.HTTPErrorCodeReturnedEquals,
			},
			Redirect: xmlRoutingRedirect{
				HostName:             rule.Redirect.HostName,
				Protocol:             rule.Redirect.Protocol,
				ReplaceKeyPrefixWith: rule.Redirect.ReplaceKeyPrefixWith,
				ReplaceKeyWith:       rule.Redirect.ReplaceKeyWith,
				HTTPRedirectCode:     rule.Redirect.HTTPRedirectCode,
			},
		})
	}
	writeXML(w, r, http.StatusOK, resp)
}

// deleteBucketWebsite handles DELETE /{bucket}?website.
func (h *Handler) deleteBucketWebsite(w http.ResponseWriter, r *http.Request, bucket string) {
	if err := h.engine.SetBucketWebsite(r.Context(), bucket, nil); err != nil {
		if isBucketNotFound(err) {
			writeError(w, r, http.StatusNotFound, "NoSuchBucket", "The specified bucket does not exist")
			return
		}
		writeError(w, r, http.StatusInternalServerError, "InternalError", err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// --- XML types for website configuration ---

type xmlWebsiteConfiguration struct {
	XMLName       xml.Name         `xml:"WebsiteConfiguration"`
	IndexDocument xmlIndexDocument `xml:"IndexDocument"`
	ErrorDocument xmlErrorDocument `xml:"ErrorDocument"`
	RoutingRules  []xmlRoutingRule `xml:"RoutingRules>RoutingRule"`
}

type xmlIndexDocument struct {
	Suffix string `xml:"Suffix"`
}

type xmlErrorDocument struct {
	Key string `xml:"Key"`
}

type xmlRoutingRule struct {
	Condition xmlRoutingCondition `xml:"Condition"`
	Redirect  xmlRoutingRedirect  `xml:"Redirect"`
}

type xmlRoutingCondition struct {
	KeyPrefixEquals             string `xml:"KeyPrefixEquals,omitempty"`
	HTTPErrorCodeReturnedEquals string `xml:"HttpErrorCodeReturnedEquals,omitempty"`
}

type xmlRoutingRedirect struct {
	HostName             string `xml:"HostName,omitempty"`
	Protocol             string `xml:"Protocol,omitempty"`
	ReplaceKeyPrefixWith string `xml:"ReplaceKeyPrefixWith,omitempty"`
	ReplaceKeyWith       string `xml:"ReplaceKeyWith,omitempty"`
	HTTPRedirectCode     string `xml:"HttpRedirectCode,omitempty"`
}
