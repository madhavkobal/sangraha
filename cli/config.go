package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/madhavkobal/sangraha/internal/config"
)

// configCmd is the parent command for configuration operations.
var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage server configuration",
}

var configShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Show the current effective configuration",
	RunE: func(_ *cobra.Command, _ []string) error {
		cfg, err := config.Load(flagConfigFile)
		if err != nil {
			return fmt.Errorf("load config: %w", err)
		}
		fmt.Printf("s3_address:    %s\n", cfg.Server.S3Address)
		fmt.Printf("admin_address: %s\n", cfg.Server.AdminAddress)
		fmt.Printf("data_dir:      %s\n", cfg.Storage.DataDir)
		fmt.Printf("metadata:      %s\n", cfg.Metadata.Path)
		fmt.Printf("backend:       %s\n", cfg.Storage.Backend)
		fmt.Printf("log_level:     %s\n", cfg.Logging.Level)
		fmt.Printf("tls_enabled:   %v\n", cfg.Server.TLS.Enabled)
		return nil
	},
}

var configValidateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Parse and validate the configuration file",
	RunE: func(_ *cobra.Command, _ []string) error {
		cfg, err := config.Load(flagConfigFile)
		if err != nil {
			return fmt.Errorf("load config: %w", err)
		}
		if err = config.Validate(cfg); err != nil {
			return fmt.Errorf("validation failed: %w", err)
		}
		fmt.Println("configuration is valid")
		return nil
	},
}

func init() {
	configShowCmd.Flags().StringVar(&flagConfigFile, "config", "", "path to config file")
	configValidateCmd.Flags().StringVar(&flagConfigFile, "config", "", "path to config file")
	configCmd.AddCommand(configShowCmd, configValidateCmd)
}
