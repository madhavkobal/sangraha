package cli

import (
	"bufio"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

// initCmd performs first-time setup: config, data directories, root credentials.
var initCmd = &cobra.Command{
	Use:   "init",
	Short: "First-time setup: create config, data directories, and root credentials",
	RunE:  runInit,
}

func init() {
	rootCmd.AddCommand(initCmd)
}

// initConfig holds the answers collected during the init wizard.
type initConfig struct {
	configDir     string
	dataDir       string
	metaPath      string
	s3Addr        string
	adminAddr     string
	rootAccessKey string
	rootSecretKey string
	sseKey        string
	tlsEnabled    bool
	autoSelfSign  bool
	certFile      string
	keyFile       string
}

func runInit(_ *cobra.Command, _ []string) error {
	reader := bufio.NewReader(os.Stdin)

	fmt.Println("=== sangraha init wizard ===")
	fmt.Println()

	cfg, err := collectInitAnswers(reader)
	if err != nil {
		return err
	}

	return writeInitFiles(cfg)
}

// collectInitAnswers interactively prompts the user and returns the filled initConfig.
func collectInitAnswers(reader *bufio.Reader) (initConfig, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		homeDir = "."
	}

	cfg := initConfig{}

	cfg.configDir = promptDefault(reader, "Config directory", filepath.Join(homeDir, ".sangraha"))
	cfg.configDir = filepath.Clean(cfg.configDir) //nolint:gosec // G304: path is operator-provided
	err = os.MkdirAll(cfg.configDir, 0700)
	if err != nil {
		return initConfig{}, fmt.Errorf("create config dir: %w", err)
	}

	cfg.dataDir = promptDefault(reader, "Data directory", filepath.Join(cfg.configDir, "data"))
	cfg.dataDir = filepath.Clean(cfg.dataDir) //nolint:gosec // G304: path is operator-provided
	err = os.MkdirAll(cfg.dataDir, 0750)
	if err != nil {
		return initConfig{}, fmt.Errorf("create data dir: %w", err)
	}

	cfg.metaPath = promptDefault(reader, "Metadata DB path", filepath.Join(cfg.configDir, "meta.db"))
	cfg.metaPath = filepath.Clean(cfg.metaPath)
	cfg.s3Addr = promptDefault(reader, "S3 API listen address", ":9000")
	cfg.adminAddr = promptDefault(reader, "Admin API listen address", ":9001")
	cfg.rootAccessKey = promptDefault(reader, "Root access key", "root")

	cfg.rootSecretKey, err = promptOrGenerate(reader, "Root secret key", 32)
	if err != nil {
		return initConfig{}, fmt.Errorf("generate secret: %w", err)
	}

	cfg.sseKey, err = promptOrGenerate(reader, "Server-side encryption master key", 32)
	if err != nil {
		return initConfig{}, fmt.Errorf("generate SSE key: %w", err)
	}

	cfg.tlsEnabled = promptYesNo(reader, "Enable TLS? [y/N]: ")
	if cfg.tlsEnabled {
		cfg.autoSelfSign = promptYesNo(reader, "Auto-generate self-signed certificate? [Y/n]: ")
		if !cfg.autoSelfSign {
			cfg.certFile = prompt(reader, "TLS certificate file path: ")
			cfg.keyFile = prompt(reader, "TLS key file path: ")
		}
	}

	return cfg, nil
}

// writeInitFiles writes config.yaml and the env credential file.
func writeInitFiles(cfg initConfig) error {
	cfgPath := filepath.Join(cfg.configDir, "config.yaml")
	cfgContent := buildConfig(cfg)
	//nolint:gosec // G306: config file contains no secrets; credentials go in env file (0600)
	if err := os.WriteFile(cfgPath, []byte(cfgContent), 0600); err != nil {
		return fmt.Errorf("write config: %w", err)
	}

	envPath := filepath.Join(cfg.configDir, "env")
	envContent := fmt.Sprintf(
		"export SANGRAHA_ROOT_SECRET_KEY=%s\nexport SANGRAHA_SSE_MASTER_KEY=%s\n",
		cfg.rootSecretKey, cfg.sseKey,
	)
	//nolint:gosec // G703: envPath is operator-provided and sanitised with filepath.Join + Clean
	if err := os.WriteFile(envPath, []byte(envContent), 0600); err != nil {
		return fmt.Errorf("write env file: %w", err)
	}

	fmt.Println()
	fmt.Println("=== Setup complete ===")
	fmt.Printf("Config file:  %s\n", cfgPath)
	fmt.Printf("Credentials:  %s\n", envPath)
	fmt.Printf("Data dir:     %s\n", cfg.dataDir)
	fmt.Printf("Metadata DB:  %s\n", cfg.metaPath)
	fmt.Println()
	fmt.Println("To start the server:")
	fmt.Printf("  source %s\n", envPath)
	fmt.Printf("  SANGRAHA_CONFIG=%s sangraha server start\n", cfgPath)
	return nil
}

func prompt(reader *bufio.Reader, label string) string {
	fmt.Print(label)
	text, _ := reader.ReadString('\n')
	return strings.TrimSpace(text)
}

func promptDefault(reader *bufio.Reader, label, def string) string {
	v := prompt(reader, fmt.Sprintf("%s [%s]: ", label, def))
	if v == "" {
		return def
	}
	return v
}

func promptYesNo(reader *bufio.Reader, label string) bool {
	answer := strings.ToLower(prompt(reader, label))
	return answer == "y" || answer == "yes"
}

// promptOrGenerate prompts for a secret value; if left blank, generates n random bytes.
func promptOrGenerate(reader *bufio.Reader, label string, n int) (string, error) {
	v := prompt(reader, label+" (leave blank to generate): ")
	if v != "" {
		return v, nil
	}
	secret, err := generateSecret(n)
	if err != nil {
		return "", err
	}
	fmt.Printf("Generated %s: %s\n", strings.ToLower(label), secret)
	fmt.Println("IMPORTANT: Save this key — it will not be shown again.")
	fmt.Println()
	return secret, nil
}

func generateSecret(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func buildConfig(cfg initConfig) string {
	tlsSection := fmt.Sprintf(`  tls:
    enabled: %v
    auto_self_signed: %v
    cert_file: "%s"
    key_file: "%s"
`, cfg.tlsEnabled, cfg.autoSelfSign, cfg.certFile, cfg.keyFile)

	return fmt.Sprintf(`# sangraha configuration — generated by 'sangraha init'
server:
  s3_address: "%s"
  admin_address: "%s"
%s
storage:
  backend: localfs
  data_dir: "%s"

metadata:
  path: "%s"

auth:
  root_access_key: "%s"
  # root_secret_key is set via the SANGRAHA_ROOT_SECRET_KEY environment variable.

logging:
  level: info
  format: json

limits:
  max_bucket_count: 1000
  rate_limit_rps: 1000
`, cfg.s3Addr, cfg.adminAddr, tlsSection, cfg.dataDir, cfg.metaPath, cfg.rootAccessKey)
}
