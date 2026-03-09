// Package config loads and validates the sangraha configuration from YAML
// files, TOML files, and environment variables via viper.
package config

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/viper"

	"github.com/madhavkobal/sangraha/internal/auth"
	"github.com/madhavkobal/sangraha/internal/backend/tiered"
	"github.com/madhavkobal/sangraha/internal/cluster"
)

// Config is the top-level configuration structure for the sangraha server.
type Config struct {
	Server   ServerConfig   `mapstructure:"server"`
	Storage  StorageConfig  `mapstructure:"storage"`
	Metadata MetadataConfig `mapstructure:"metadata"`
	Auth     AuthConfig     `mapstructure:"auth"`
	Logging  LoggingConfig  `mapstructure:"logging"`
	Limits   LimitsConfig   `mapstructure:"limits"`
	Cluster  ClusterConfig  `mapstructure:"cluster"`
}

// ServerConfig holds network and TLS settings.
type ServerConfig struct {
	S3Address    string    `mapstructure:"s3_address"`
	AdminAddress string    `mapstructure:"admin_address"`
	TLS          TLSConfig `mapstructure:"tls"`
}

// TLSConfig holds TLS certificate settings.
type TLSConfig struct {
	Enabled        bool   `mapstructure:"enabled"`
	CertFile       string `mapstructure:"cert_file"`
	KeyFile        string `mapstructure:"key_file"`
	AutoSelfSigned bool   `mapstructure:"auto_self_signed"`
}

// StorageConfig controls the storage backend.
type StorageConfig struct {
	Backend string              `mapstructure:"backend"`
	DataDir string              `mapstructure:"data_dir"`
	Tiers   []tiered.TierConfig `mapstructure:"tiers"`
}

// MetadataConfig controls the metadata store location.
type MetadataConfig struct {
	Path string `mapstructure:"path"`
}

// AuthConfig holds authentication settings.
type AuthConfig struct {
	RootAccessKey string          `mapstructure:"root_access_key"`
	// RootSecretKey must be set via the SANGRAHA_ROOT_SECRET_KEY env var only.
	RootSecretKey string          `mapstructure:"-"`
	OIDC          auth.OIDCConfig `mapstructure:"oidc"`
	LDAP          auth.LDAPConfig `mapstructure:"ldap"`
}

// ClusterConfig controls multi-node clustering behaviour.
type ClusterConfig struct {
	// Enabled activates cluster mode. When false (default) the node runs in
	// single-node mode and is always the leader.
	Enabled bool               `mapstructure:"enabled"`
	Node    cluster.NodeConfig `mapstructure:"node"`
}

// LoggingConfig controls log output.
type LoggingConfig struct {
	Level    string `mapstructure:"level"`
	Format   string `mapstructure:"format"`
	AuditLog string `mapstructure:"audit_log"`
}

// LimitsConfig enforces server-wide resource limits.
type LimitsConfig struct {
	MaxObjectSize  string        `mapstructure:"max_object_size"`
	MaxBucketCount int           `mapstructure:"max_bucket_count"`
	RateLimitRPS   int           `mapstructure:"rate_limit_rps"`
	ReadTimeout    time.Duration `mapstructure:"read_timeout"`
	WriteTimeout   time.Duration `mapstructure:"write_timeout"`
	IdleTimeout    time.Duration `mapstructure:"idle_timeout"`
}

// Load reads the configuration file at the given path plus environment
// variable overrides and returns the parsed Config. If path is empty the
// viper search paths are used.
func Load(path string) (*Config, error) {
	v := viper.New()
	setDefaults(v)

	v.SetEnvPrefix("SANGRAHA")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	if path != "" {
		v.SetConfigFile(path)
	} else {
		v.SetConfigName("config")
		v.SetConfigType("yaml")
		v.AddConfigPath("$HOME/.sangraha")
		v.AddConfigPath("/etc/sangraha")
		v.AddConfigPath(".")
	}

	if err := v.ReadInConfig(); err != nil {
		// Config file is optional; missing file is not an error.
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("read config: %w", err)
		}
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("unmarshal config: %w", err)
	}

	// Secret key must come from env only — never from config file.
	cfg.Auth.RootSecretKey = v.GetString("ROOT_SECRET_KEY")

	return &cfg, nil
}

// Validate checks that the loaded configuration is internally consistent.
func Validate(cfg *Config) error {
	if cfg.Server.S3Address == "" {
		return fmt.Errorf("server.s3_address must not be empty")
	}
	if cfg.Server.AdminAddress == "" {
		return fmt.Errorf("server.admin_address must not be empty")
	}
	if cfg.Storage.DataDir == "" {
		return fmt.Errorf("storage.data_dir must not be empty")
	}
	if cfg.Metadata.Path == "" {
		return fmt.Errorf("metadata.path must not be empty")
	}
	switch cfg.Storage.Backend {
	case "localfs", "badger", "tiered":
		// valid
	default:
		return fmt.Errorf("storage.backend %q is not supported; choose localfs, badger, or tiered", cfg.Storage.Backend)
	}
	if cfg.Storage.Backend == "tiered" && len(cfg.Storage.Tiers) == 0 {
		return fmt.Errorf("storage.backend is \"tiered\" but storage.tiers is empty")
	}
	if cfg.Auth.RootAccessKey == "" {
		return fmt.Errorf("auth.root_access_key must not be empty")
	}
	return nil
}
