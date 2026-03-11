package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"

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
	RunE:  runUserCreate,
}

var userDeleteCmd = &cobra.Command{
	Use:   "delete <access-key>",
	Short: "Delete a user by access key",
	Args:  cobra.ExactArgs(1),
	RunE:  runUserDelete,
}

var userListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all users",
	RunE:  runUserList,
}

var userRotateKeyCmd = &cobra.Command{
	Use:   "rotate-key <access-key>",
	Short: "Rotate the access key for a user",
	Args:  cobra.ExactArgs(1),
	RunE:  runUserRotateKey,
}

func init() {
	userDeleteCmd.Flags().Bool("force", false, "skip confirmation prompt")
	userCmd.AddCommand(userCreateCmd, userDeleteCmd, userListCmd, userRotateKeyCmd)
}

// createUserResponse mirrors the admin API createUserResponse.
type createUserResponse struct {
	AccessKey string `json:"access_key"` //nolint:gosec // G101/G117: access_key is an identifier, not a credential
	SecretKey string `json:"secret_key"`
	Owner     string `json:"owner"`
}

// userInfo mirrors the admin API user listing entry.
type userInfo struct {
	AccessKey string `json:"access_key"` //nolint:gosec // G101/G117: access_key is an identifier, not a credential
	Owner     string `json:"owner"`
	IsRoot    bool   `json:"is_root"`
}

func runUserCreate(_ *cobra.Command, args []string) error {
	username := args[0]

	result, err := adminJSON[createUserResponse]("POST", "/admin/v1/users", map[string]string{"owner": username})
	if err != nil {
		return err
	}

	if flagJSON {
		return json.NewEncoder(os.Stdout).Encode(map[string]string{
			"access_key": result.AccessKey,
			"owner":      result.Owner,
		})
	}

	fmt.Printf("User created successfully.\n\n")
	fmt.Printf("Owner:      %s\n", result.Owner)
	fmt.Printf("Access Key: %s\n", result.AccessKey)
	fmt.Printf("Secret Key: %s\n", result.SecretKey)
	fmt.Println()
	fmt.Println("IMPORTANT: Save the secret key — it will not be shown again.")
	return nil
}

func runUserDelete(cmd *cobra.Command, args []string) error {
	accessKey := args[0]

	force, _ := cmd.Flags().GetBool("force")
	if !force {
		if !confirmAction(fmt.Sprintf("delete user with access key %q", accessKey)) {
			return nil
		}
	}

	resp, err := adminDo("DELETE", "/admin/v1/users/"+accessKey, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode >= 400 {
		return fmt.Errorf("delete user returned HTTP %d", resp.StatusCode)
	}
	fmt.Printf("User %s deleted.\n", accessKey)
	return nil
}

func runUserList(_ *cobra.Command, _ []string) error {
	users, err := adminJSON[[]userInfo]("GET", "/admin/v1/users", nil)
	if err != nil {
		return err
	}

	if flagJSON {
		return json.NewEncoder(os.Stdout).Encode(users) //nolint:gosec // G117: access_key is an identifier, not a credential
	}

	tw := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	if _, err := fmt.Fprintln(tw, "ACCESS KEY\tOWNER\tROOT"); err != nil {
		return err
	}
	for _, u := range users {
		root := ""
		if u.IsRoot {
			root = "yes"
		}
		if _, err := fmt.Fprintf(tw, "%s\t%s\t%s\n", u.AccessKey, u.Owner, root); err != nil {
			return err
		}
	}
	return tw.Flush()
}

func runUserRotateKey(_ *cobra.Command, args []string) error {
	accessKey := args[0]

	result, err := adminJSON[createUserResponse]("POST", "/admin/v1/users/"+accessKey+"/keys/rotate", nil)
	if err != nil {
		return err
	}

	if flagJSON {
		return json.NewEncoder(os.Stdout).Encode(map[string]string{
			"access_key": result.AccessKey,
			"owner":      result.Owner,
		})
	}

	fmt.Printf("Key rotated for user %q.\n\n", result.Owner)
	fmt.Printf("New Access Key: %s\n", result.AccessKey)
	fmt.Printf("New Secret Key: %s\n", result.SecretKey)
	fmt.Println()
	fmt.Println("IMPORTANT: Save the new secret key — it will not be shown again.")
	return nil
}

// confirmAction prompts the user to confirm a destructive action.
// Returns true if confirmed.
func confirmAction(action string) bool {
	fmt.Printf("Are you sure you want to %s? [y/N]: ", action)
	var answer string
	if _, err := fmt.Scanln(&answer); err != nil {
		return false
	}
	return answer == "y" || answer == "Y" || answer == "yes"
}
