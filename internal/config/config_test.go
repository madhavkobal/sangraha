package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadDefaults(t *testing.T) {
	// Load with no file — should use defaults without error.
	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load(\"\") unexpected error: %v", err)
	}
	if cfg.Server.S3Address != ":9000" {
		t.Errorf("S3Address = %q; want :9000", cfg.Server.S3Address)
	}
	if cfg.Server.AdminAddress != ":9001" {
		t.Errorf("AdminAddress = %q; want :9001", cfg.Server.AdminAddress)
	}
	if cfg.Storage.Backend != "localfs" {
		t.Errorf("Backend = %q; want localfs", cfg.Storage.Backend)
	}
}

func TestLoadFromFile(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	content := `
server:
  s3_address: ":19000"
  admin_address: ":19001"
storage:
  backend: localfs
  data_dir: /tmp/test-data
metadata:
  path: /tmp/test-meta.db
auth:
  root_access_key: testroot
logging:
  level: debug
`
	if err := os.WriteFile(cfgPath, []byte(content), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Server.S3Address != ":19000" {
		t.Errorf("S3Address = %q; want :19000", cfg.Server.S3Address)
	}
	if cfg.Logging.Level != "debug" {
		t.Errorf("Level = %q; want debug", cfg.Logging.Level)
	}
	if cfg.Auth.RootAccessKey != "testroot" {
		t.Errorf("RootAccessKey = %q; want testroot", cfg.Auth.RootAccessKey)
	}
}

func TestValidate(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		cfg, _ := Load("")
		cfg.Storage.DataDir = "/tmp"
		cfg.Metadata.Path = "/tmp/meta.db"
		cfg.Auth.RootAccessKey = "root"
		if err := Validate(cfg); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("missing s3 address", func(t *testing.T) {
		cfg, _ := Load("")
		cfg.Server.S3Address = ""
		if err := Validate(cfg); err == nil {
			t.Error("expected error, got nil")
		}
	})

	t.Run("invalid backend", func(t *testing.T) {
		cfg, _ := Load("")
		cfg.Storage.DataDir = "/tmp"
		cfg.Metadata.Path = "/tmp/meta.db"
		cfg.Auth.RootAccessKey = "root"
		cfg.Storage.Backend = "invalid"
		if err := Validate(cfg); err == nil {
			t.Error("expected error for invalid backend, got nil")
		}
	})
}

func TestEnvVarOverride(t *testing.T) {
	t.Setenv("SANGRAHA_SERVER_S3_ADDRESS", ":29000")
	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Server.S3Address != ":29000" {
		t.Errorf("S3Address = %q; want :29000 (from env)", cfg.Server.S3Address)
	}
}
