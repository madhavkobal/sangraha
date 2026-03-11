package middleware

import (
	"crypto/tls"
	"os"
	"path/filepath"
	"testing"
)

func TestBuildTLSConfigAutoSelfSigned(t *testing.T) {
	cfg, err := BuildTLSConfig("", "", true)
	if err != nil {
		t.Fatalf("BuildTLSConfig auto self-signed: %v", err)
	}
	if cfg == nil {
		t.Fatal("BuildTLSConfig should return non-nil config")
	}
	if cfg.MinVersion != tls.VersionTLS12 {
		t.Errorf("MinVersion = %d; want TLS 1.2 (%d)", cfg.MinVersion, tls.VersionTLS12)
	}
	if len(cfg.Certificates) == 0 {
		t.Error("should have at least one certificate")
	}
}

func TestBuildTLSConfigFromFiles(t *testing.T) {
	dir := t.TempDir()
	certFile := filepath.Join(dir, "tls.crt")
	keyFile := filepath.Join(dir, "tls.key")

	if err := WriteSelfSignedCerts(certFile, keyFile); err != nil {
		t.Fatalf("WriteSelfSignedCerts: %v", err)
	}

	cfg, err := BuildTLSConfig(certFile, keyFile, false)
	if err != nil {
		t.Fatalf("BuildTLSConfig from files: %v", err)
	}
	if cfg == nil {
		t.Fatal("config should not be nil")
	}
}

func TestBuildTLSConfigNoCredentials(t *testing.T) {
	_, err := BuildTLSConfig("", "", false)
	if err == nil {
		t.Error("should return error when no cert/key and auto is disabled")
	}
}

func TestBuildTLSConfigMissingFiles(t *testing.T) {
	_, err := BuildTLSConfig("/nonexistent/cert.pem", "/nonexistent/key.pem", false)
	if err == nil {
		t.Error("should return error when cert files are missing")
	}
}

func TestWriteSelfSignedCerts(t *testing.T) {
	dir := t.TempDir()
	certFile := filepath.Join(dir, "cert.pem")
	keyFile := filepath.Join(dir, "key.pem")

	if err := WriteSelfSignedCerts(certFile, keyFile); err != nil {
		t.Fatalf("WriteSelfSignedCerts: %v", err)
	}

	// Verify the output files can be loaded as TLS keypair.
	_, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		t.Errorf("LoadX509KeyPair: %v", err)
	}

	// Verify file permissions.
	info, err := os.Stat(certFile)
	if err != nil {
		t.Fatalf("stat certFile: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("cert file permissions = %o; want 600", perm)
	}
}

func TestWriteSelfSignedCertsInvalidPath(t *testing.T) {
	err := WriteSelfSignedCerts("/nonexistent-dir/cert.pem", "/nonexistent-dir/key.pem")
	if err == nil {
		t.Error("should return error for invalid directory")
	}
}
