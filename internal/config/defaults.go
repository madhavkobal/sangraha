package config

import (
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/viper"
)

func setDefaults(v *viper.Viper) {
	home, _ := os.UserHomeDir()
	dataDir := filepath.Join(home, ".sangraha", "data")
	metaPath := filepath.Join(home, ".sangraha", "meta.db")

	v.SetDefault("server.s3_address", ":9000")
	v.SetDefault("server.admin_address", ":9001")
	v.SetDefault("server.tls.enabled", true)
	v.SetDefault("server.tls.auto_self_signed", true)
	v.SetDefault("server.tls.cert_file", filepath.Join(home, ".sangraha", "tls.crt"))
	v.SetDefault("server.tls.key_file", filepath.Join(home, ".sangraha", "tls.key"))

	v.SetDefault("storage.backend", "localfs")
	v.SetDefault("storage.data_dir", dataDir)

	v.SetDefault("metadata.path", metaPath)

	v.SetDefault("auth.root_access_key", "root")

	v.SetDefault("logging.level", "info")
	v.SetDefault("logging.format", "json")
	v.SetDefault("logging.audit_log", filepath.Join(home, ".sangraha", "audit.log"))

	v.SetDefault("limits.max_object_size", "5TB")
	v.SetDefault("limits.max_bucket_count", 1000)
	v.SetDefault("limits.rate_limit_rps", 1000)
	v.SetDefault("limits.read_timeout", (30 * time.Second).String())
	v.SetDefault("limits.write_timeout", (30 * time.Second).String())
	v.SetDefault("limits.idle_timeout", (120 * time.Second).String())
}
