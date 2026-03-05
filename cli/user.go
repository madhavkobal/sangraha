package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

// userCmd is the parent command for user management.
var userCmd = &cobra.Command{
	Use:   "user",
	Short: "Manage users and access keys",
}

var userCreateCmd = &cobra.Command{
	Use:   "create <username>",
	Short: "Create a user and generate an access key pair",
	Args:  cobra.ExactArgs(1),
	RunE: func(_ *cobra.Command, _ []string) error {
		return fmt.Errorf("user create: not yet implemented (Phase 1 CLI stub)")
	},
}

var userDeleteCmd = &cobra.Command{
	Use:   "delete <access-key>",
	Short: "Delete a user by access key",
	Args:  cobra.ExactArgs(1),
	RunE: func(_ *cobra.Command, _ []string) error {
		return fmt.Errorf("user delete: not yet implemented (Phase 1 CLI stub)")
	},
}

var userListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all users",
	RunE: func(_ *cobra.Command, _ []string) error {
		return fmt.Errorf("user list: not yet implemented (Phase 1 CLI stub)")
	},
}

var userRotateKeyCmd = &cobra.Command{
	Use:   "rotate-key <access-key>",
	Short: "Rotate the access key for a user",
	Args:  cobra.ExactArgs(1),
	RunE: func(_ *cobra.Command, _ []string) error {
		return fmt.Errorf("user rotate-key: not yet implemented (Phase 1 CLI stub)")
	},
}

func init() {
	userCmd.AddCommand(userCreateCmd, userDeleteCmd, userListCmd, userRotateKeyCmd)
}
