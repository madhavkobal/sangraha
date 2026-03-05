package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

// objectCmd is the parent command for object operations.
var objectCmd = &cobra.Command{
	Use:   "object",
	Short: "Manage objects",
}

var objectPutCmd = &cobra.Command{
	Use:   "put <bucket> <key> <file>",
	Short: "Upload an object",
	Args:  cobra.ExactArgs(3),
	RunE: func(_ *cobra.Command, _ []string) error {
		return fmt.Errorf("object put: not yet implemented (Phase 1 CLI stub)")
	},
}

var objectGetCmd = &cobra.Command{
	Use:   "get <bucket> <key>",
	Short: "Download an object",
	Args:  cobra.ExactArgs(2),
	RunE: func(_ *cobra.Command, _ []string) error {
		return fmt.Errorf("object get: not yet implemented (Phase 1 CLI stub)")
	},
}

var objectDeleteCmd = &cobra.Command{
	Use:   "delete <bucket> <key>",
	Short: "Delete an object",
	Args:  cobra.ExactArgs(2),
	RunE: func(_ *cobra.Command, _ []string) error {
		return fmt.Errorf("object delete: not yet implemented (Phase 1 CLI stub)")
	},
}

var objectListCmd = &cobra.Command{
	Use:   "list <bucket>",
	Short: "List objects in a bucket",
	Args:  cobra.ExactArgs(1),
	RunE: func(_ *cobra.Command, _ []string) error {
		return fmt.Errorf("object list: not yet implemented (Phase 1 CLI stub)")
	},
}

func init() {
	objectGetCmd.Flags().StringP("output", "o", "", "write output to file instead of stdout")
	objectPutCmd.Flags().String("content-type", "", "object content type")
	objectListCmd.Flags().String("prefix", "", "filter by prefix")
	objectListCmd.Flags().String("delimiter", "", "grouping delimiter")
	objectCmd.AddCommand(objectPutCmd, objectGetCmd, objectDeleteCmd, objectListCmd)
}
