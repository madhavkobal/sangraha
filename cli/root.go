// Package cli implements the sangraha command-line interface using cobra.
// The CLI communicates with a running sangraha server via its S3 and Admin
// APIs. All commands read server address and credentials from environment
// variables or flags.
package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	// flagServer is the S3 API address.
	flagServer string
	// flagAdminURL is the admin API address.
	flagAdminURL string
	// flagAccessKey is the S3 access key.
	flagAccessKey string
	// flagSecretKey is the S3 secret key.
	flagSecretKey string
	// flagJSON switches all output to machine-readable JSON.
	flagJSON bool
	// flagInsecure skips TLS certificate verification (operator use only).
	flagInsecure bool
)

// rootCmd is the base command.
var rootCmd = &cobra.Command{
	Use:   "sangraha",
	Short: "S3-compatible object storage",
	Long: `sangraha is a single-binary, S3-compatible object storage system.

Run 'sangraha --help' for a list of available commands.`,
}

// Execute runs the root command and exits with a non-zero status on failure.
func Execute(version, buildTime string) {
	rootCmd.Version = fmt.Sprintf("%s (built %s)", version, buildTime)
	binaryVersion = version
	binaryBuildTime = buildTime
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVar(&flagServer, "server",
		envOrDefault("SANGRAHA_SERVER", "https://localhost:9000"),
		"sangraha S3 API address ($SANGRAHA_SERVER)")
	rootCmd.PersistentFlags().StringVar(&flagAdminURL, "admin-url",
		envOrDefault("SANGRAHA_ADMIN_URL", "https://localhost:9001"),
		"sangraha admin API address ($SANGRAHA_ADMIN_URL)")
	rootCmd.PersistentFlags().StringVar(&flagAccessKey, "access-key",
		envOrDefault("SANGRAHA_ACCESS_KEY", ""),
		"S3 access key ($SANGRAHA_ACCESS_KEY)")
	rootCmd.PersistentFlags().StringVar(&flagSecretKey, "secret-key",
		envOrDefault("SANGRAHA_SECRET_KEY", ""),
		"S3 secret key ($SANGRAHA_SECRET_KEY)")
	rootCmd.PersistentFlags().BoolVar(&flagJSON, "json", false, "output JSON instead of human-readable tables")
	rootCmd.PersistentFlags().BoolVar(&flagInsecure, "insecure", false, "skip TLS certificate verification")

	rootCmd.AddCommand(serverCmd)
	rootCmd.AddCommand(bucketCmd)
	rootCmd.AddCommand(objectCmd)
	rootCmd.AddCommand(userCmd)
	rootCmd.AddCommand(configCmd)
	rootCmd.AddCommand(adminCmd)
}

func envOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
