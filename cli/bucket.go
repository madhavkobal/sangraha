package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

// bucketCmd is the parent command for bucket operations.
var bucketCmd = &cobra.Command{
	Use:   "bucket",
	Short: "Manage buckets",
}

var bucketCreateCmd = &cobra.Command{
	Use:   "create <name>",
	Short: "Create a bucket",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return fmt.Errorf("bucket create: server communication not yet implemented (Phase 1 CLI stub)")
	},
}

var bucketDeleteCmd = &cobra.Command{
	Use:   "delete <name>",
	Short: "Delete a bucket",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return fmt.Errorf("bucket delete: server communication not yet implemented (Phase 1 CLI stub)")
	},
}

var bucketListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all buckets",
	RunE: func(cmd *cobra.Command, _ []string) error {
		// Stub: print help until HTTP client is wired in Phase 1.6.
		type bucketInfo struct {
			Name string `json:"name"`
		}
		buckets := []bucketInfo{}

		if flagJSON {
			return json.NewEncoder(os.Stdout).Encode(buckets)
		}
		tw := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		if _, err := fmt.Fprintln(tw, "NAME\tCREATED"); err != nil {
			return err
		}
		return tw.Flush()
	},
}

func init() {
	bucketDeleteCmd.Flags().Bool("force", false, "skip confirmation prompt")
	bucketCmd.AddCommand(bucketCreateCmd, bucketDeleteCmd, bucketListCmd)
}
