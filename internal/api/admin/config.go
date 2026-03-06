package admin

import (
	"encoding/json"
	"net/http"
	"strings"
	"sync"

	"github.com/madhavkobal/sangraha/internal/config"
)

// configHandler serves config read/write/validate endpoints.
type configHandler struct {
	mu  sync.RWMutex
	cfg *config.Config
}

// safeConfig is the JSON representation of Config with secrets masked.
type safeConfig struct {
	Server   safeServerConfig  `json:"server"`
	Storage  safeStorageConfig `json:"storage"`
	Metadata safeMetaConfig    `json:"metadata"`
	Auth     safeAuthConfig    `json:"auth"`
	Logging  safeLoggingConfig `json:"logging"`
	Limits   safeLimitsConfig  `json:"limits"`
}

type safeServerConfig struct {
	S3Address    string        `json:"s3_address"`
	AdminAddress string        `json:"admin_address"`
	TLS          safeTLSConfig `json:"tls"`
}

type safeTLSConfig struct {
	Enabled        bool   `json:"enabled"`
	CertFile       string `json:"cert_file"`
	KeyFile        string `json:"key_file"`
	AutoSelfSigned bool   `json:"auto_self_signed"`
}

type safeStorageConfig struct {
	Backend string `json:"backend"`
	DataDir string `json:"data_dir"`
}

type safeMetaConfig struct {
	Path string `json:"path"`
}

type safeAuthConfig struct {
	RootAccessKey string `json:"root_access_key"`
	RootSecretKey string `json:"root_secret_key"` // always "***"
}

type safeLoggingConfig struct {
	Level    string `json:"level"`
	Format   string `json:"format"`
	AuditLog string `json:"audit_log"`
}

type safeLimitsConfig struct {
	MaxObjectSize  string `json:"max_object_size"`
	MaxBucketCount int    `json:"max_bucket_count"`
	RateLimitRPS   int    `json:"rate_limit_rps"`
}

// configPatch is a partial update document for PUT /admin/v1/config.
type configPatch struct {
	Logging *struct {
		Level  *string `json:"level"`
		Format *string `json:"format"`
	} `json:"logging"`
	Limits *struct {
		RateLimitRPS   *int `json:"rate_limit_rps"`
		MaxBucketCount *int `json:"max_bucket_count"`
	} `json:"limits"`
}

// configValidateResponse is returned by POST /admin/v1/config/validate.
type configValidateResponse struct {
	Valid  bool     `json:"valid"`
	Errors []string `json:"errors,omitempty"`
}

// configUpdateResponse is returned by PUT /admin/v1/config.
type configUpdateResponse struct {
	Applied         bool   `json:"applied"`
	RestartRequired bool   `json:"restart_required"`
	Message         string `json:"message"`
}

func (h *configHandler) get(w http.ResponseWriter, _ *http.Request) {
	h.mu.RLock()
	cfg := h.cfg
	h.mu.RUnlock()
	writeJSON(w, http.StatusOK, toSafeConfig(cfg))
}

func (h *configHandler) validate(w http.ResponseWriter, r *http.Request) {
	var patch configPatch
	if err := json.NewDecoder(r.Body).Decode(&patch); err != nil {
		writeJSON(w, http.StatusBadRequest, configValidateResponse{
			Valid:  false,
			Errors: []string{"invalid JSON: " + err.Error()},
		})
		return
	}

	var errs []string
	if patch.Logging != nil && patch.Logging.Level != nil {
		switch *patch.Logging.Level {
		case "debug", "info", "warn", "error":
		default:
			errs = append(errs, "logging.level must be one of: debug, info, warn, error")
		}
	}
	if patch.Logging != nil && patch.Logging.Format != nil {
		switch *patch.Logging.Format {
		case "json", "text":
		default:
			errs = append(errs, "logging.format must be one of: json, text")
		}
	}
	if patch.Limits != nil && patch.Limits.RateLimitRPS != nil && *patch.Limits.RateLimitRPS < 0 {
		errs = append(errs, "limits.rate_limit_rps must be >= 0")
	}
	if patch.Limits != nil && patch.Limits.MaxBucketCount != nil && *patch.Limits.MaxBucketCount < 1 {
		errs = append(errs, "limits.max_bucket_count must be >= 1")
	}

	writeJSON(w, http.StatusOK, configValidateResponse{Valid: len(errs) == 0, Errors: errs})
}

func (h *configHandler) update(w http.ResponseWriter, r *http.Request) {
	var patch configPatch
	if err := json.NewDecoder(r.Body).Decode(&patch); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON: " + err.Error()})
		return
	}

	var validationErrs []string
	if patch.Logging != nil && patch.Logging.Level != nil {
		switch *patch.Logging.Level {
		case "debug", "info", "warn", "error":
		default:
			validationErrs = append(validationErrs, "logging.level must be one of: debug, info, warn, error")
		}
	}
	if len(validationErrs) > 0 {
		writeJSON(w, http.StatusUnprocessableEntity, map[string]any{
			"error":  "validation failed",
			"fields": validationErrs,
		})
		return
	}

	h.mu.Lock()
	if patch.Logging != nil {
		if patch.Logging.Level != nil {
			h.cfg.Logging.Level = *patch.Logging.Level
		}
		if patch.Logging.Format != nil {
			h.cfg.Logging.Format = *patch.Logging.Format
		}
	}
	if patch.Limits != nil {
		if patch.Limits.RateLimitRPS != nil {
			h.cfg.Limits.RateLimitRPS = *patch.Limits.RateLimitRPS
		}
		if patch.Limits.MaxBucketCount != nil {
			h.cfg.Limits.MaxBucketCount = *patch.Limits.MaxBucketCount
		}
	}
	h.mu.Unlock()

	writeJSON(w, http.StatusOK, configUpdateResponse{
		Applied:         true,
		RestartRequired: false,
		Message:         "configuration updated; logging and limit changes take effect immediately",
	})
}

func toSafeConfig(cfg *config.Config) safeConfig {
	secretVal := "***"
	if cfg.Auth.RootSecretKey == "" {
		secretVal = "(not set)"
	}
	return safeConfig{
		Server: safeServerConfig{
			S3Address:    cfg.Server.S3Address,
			AdminAddress: cfg.Server.AdminAddress,
			TLS: safeTLSConfig{
				Enabled:        cfg.Server.TLS.Enabled,
				CertFile:       maskIfEmpty(cfg.Server.TLS.CertFile),
				KeyFile:        maskIfEmpty(cfg.Server.TLS.KeyFile),
				AutoSelfSigned: cfg.Server.TLS.AutoSelfSigned,
			},
		},
		Storage:  safeStorageConfig{Backend: cfg.Storage.Backend, DataDir: cfg.Storage.DataDir},
		Metadata: safeMetaConfig{Path: cfg.Metadata.Path},
		Auth: safeAuthConfig{
			RootAccessKey: cfg.Auth.RootAccessKey,
			RootSecretKey: secretVal,
		},
		Logging: safeLoggingConfig{
			Level:    cfg.Logging.Level,
			Format:   cfg.Logging.Format,
			AuditLog: cfg.Logging.AuditLog,
		},
		Limits: safeLimitsConfig{
			MaxObjectSize:  cfg.Limits.MaxObjectSize,
			MaxBucketCount: cfg.Limits.MaxBucketCount,
			RateLimitRPS:   cfg.Limits.RateLimitRPS,
		},
	}
}

func maskIfEmpty(s string) string {
	if strings.TrimSpace(s) == "" {
		return ""
	}
	return s
}
