package s3

import (
	"encoding/xml"
	"net/http"

	"github.com/madhavkobal/sangraha/internal/metadata"
)

// putBucketNotification handles PUT /{bucket}?notification.
func (h *Handler) putBucketNotification(w http.ResponseWriter, r *http.Request, bucket string) {
	var req xmlNotificationConfiguration
	if err := xml.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, r, http.StatusBadRequest, "MalformedXML", "invalid notification configuration XML")
		return
	}

	cfg := &metadata.NotificationConfig{}
	for _, qt := range req.QueueConfigurations {
		t := metadata.NotificationTarget{ID: qt.ID, Arn: qt.Queue, Events: qt.Events}
		if qt.Filter.S3Key.FilterRules != nil {
			t.Filter = parseXMLKeyFilter(qt.Filter)
		}
		cfg.QueueConfigurations = append(cfg.QueueConfigurations, t)
	}
	for _, tt := range req.TopicConfigurations {
		t := metadata.NotificationTarget{ID: tt.ID, Arn: tt.Topic, Events: tt.Events}
		if tt.Filter.S3Key.FilterRules != nil {
			t.Filter = parseXMLKeyFilter(tt.Filter)
		}
		cfg.TopicConfigurations = append(cfg.TopicConfigurations, t)
	}

	// If the config is completely empty, treat as a removal.
	if len(cfg.QueueConfigurations) == 0 && len(cfg.TopicConfigurations) == 0 {
		cfg = nil
	}

	if err := h.engine.SetBucketNotification(r.Context(), bucket, cfg); err != nil {
		if isBucketNotFound(err) {
			writeError(w, r, http.StatusNotFound, "NoSuchBucket", "The specified bucket does not exist")
			return
		}
		writeError(w, r, http.StatusInternalServerError, "InternalError", err.Error())
		return
	}
	w.WriteHeader(http.StatusOK)
}

// getBucketNotification handles GET /{bucket}?notification.
func (h *Handler) getBucketNotification(w http.ResponseWriter, r *http.Request, bucket string) {
	cfg, err := h.engine.GetBucketNotification(r.Context(), bucket)
	if err != nil {
		if isBucketNotFound(err) {
			writeError(w, r, http.StatusNotFound, "NoSuchBucket", "The specified bucket does not exist")
			return
		}
		writeError(w, r, http.StatusInternalServerError, "InternalError", err.Error())
		return
	}
	resp := xmlNotificationConfiguration{}
	if cfg != nil {
		for _, t := range cfg.QueueConfigurations {
			resp.QueueConfigurations = append(resp.QueueConfigurations, xmlQueueConfig{
				ID:     t.ID,
				Queue:  t.Arn,
				Events: t.Events,
				Filter: buildXMLKeyFilter(t.Filter),
			})
		}
		for _, t := range cfg.TopicConfigurations {
			resp.TopicConfigurations = append(resp.TopicConfigurations, xmlTopicConfig{
				ID:     t.ID,
				Topic:  t.Arn,
				Events: t.Events,
				Filter: buildXMLKeyFilter(t.Filter),
			})
		}
	}
	writeXML(w, r, http.StatusOK, resp)
}

// --- XML types ---

type xmlNotificationConfiguration struct {
	XMLName             xml.Name         `xml:"NotificationConfiguration"`
	QueueConfigurations []xmlQueueConfig `xml:"QueueConfiguration"`
	TopicConfigurations []xmlTopicConfig `xml:"TopicConfiguration"`
}

type xmlQueueConfig struct {
	ID     string         `xml:"Id"`
	Queue  string         `xml:"Queue"`
	Events []string       `xml:"Event"`
	Filter xmlNotifFilter `xml:"Filter"`
}

type xmlTopicConfig struct {
	ID     string         `xml:"Id"`
	Topic  string         `xml:"Topic"`
	Events []string       `xml:"Event"`
	Filter xmlNotifFilter `xml:"Filter"`
}

type xmlNotifFilter struct {
	S3Key xmlS3KeyFilter `xml:"S3Key"`
}

type xmlS3KeyFilter struct {
	FilterRules []xmlFilterRule `xml:"FilterRule"`
}

type xmlFilterRule struct {
	Name  string `xml:"Name"` // "prefix" or "suffix"
	Value string `xml:"Value"`
}

func parseXMLKeyFilter(f xmlNotifFilter) *metadata.NotificationFilter {
	nf := &metadata.NotificationFilter{}
	for _, rule := range f.S3Key.FilterRules {
		switch rule.Name {
		case "prefix":
			nf.KeyPrefixEquals = rule.Value
		case "suffix":
			nf.KeySuffixEquals = rule.Value
		}
	}
	return nf
}

func buildXMLKeyFilter(nf *metadata.NotificationFilter) xmlNotifFilter {
	if nf == nil {
		return xmlNotifFilter{}
	}
	f := xmlNotifFilter{}
	if nf.KeyPrefixEquals != "" {
		f.S3Key.FilterRules = append(f.S3Key.FilterRules, xmlFilterRule{Name: "prefix", Value: nf.KeyPrefixEquals})
	}
	if nf.KeySuffixEquals != "" {
		f.S3Key.FilterRules = append(f.S3Key.FilterRules, xmlFilterRule{Name: "suffix", Value: nf.KeySuffixEquals})
	}
	return f
}
