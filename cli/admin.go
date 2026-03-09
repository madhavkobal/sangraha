package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/spf13/cobra"
)

// adminCmd is the parent command for admin operations.
var adminCmd = &cobra.Command{
	Use:   "admin",
	Short: "Administrative operations",
}

var adminGCCmd = &cobra.Command{
	Use:   "gc",
	Short: "Trigger garbage collection of orphaned objects",
	RunE:  runAdminGC,
}

var adminExportCmd = &cobra.Command{
	Use:   "export <output-dir>",
	Short: "Export all data and metadata",
	Args:  cobra.ExactArgs(1),
	RunE:  runAdminExport,
}

var adminImportCmd = &cobra.Command{
	Use:   "import <input-dir>",
	Short: "Import data and metadata from a previous export",
	Args:  cobra.ExactArgs(1),
	RunE:  runAdminImport,
}

func init() {
	adminGCCmd.Flags().Bool("force", false, "skip confirmation prompt")
	adminCmd.AddCommand(adminGCCmd, adminExportCmd, adminImportCmd)
}

// gcTriggerResponse is the response body for POST /admin/v1/gc.
type gcTriggerResponse struct {
	Started bool   `json:"started"`
	Message string `json:"message"`
}

// gcStatusResponse is the response body for GET /admin/v1/gc/status.
type gcStatusResponse struct {
	Running   bool    `json:"running"`
	Progress  int     `json:"progress"`
	Collected int64   `json:"collected"`
	Errors    int     `json:"errors"`
	Error     string  `json:"error,omitempty"`
	StartedAt string  `json:"started_at,omitempty"`
	DoneAt    string  `json:"done_at,omitempty"`
	FreedMB   float64 `json:"freed_mb,omitempty"`
}

func runAdminGC(cmd *cobra.Command, _ []string) error {
	force, _ := cmd.Flags().GetBool("force")
	if !force {
		if !confirmAction("trigger garbage collection") {
			return nil
		}
	}

	result, err := adminJSON[gcTriggerResponse]("POST", "/admin/v1/gc", nil)
	if err != nil {
		return err
	}

	fmt.Printf("GC triggered: %s\n", result.Message)

	// Poll for completion.
	fmt.Print("Progress: ")
	for {
		status, err := adminJSON[gcStatusResponse]("GET", "/admin/v1/gc/status", nil)
		if err != nil {
			fmt.Println()
			return fmt.Errorf("poll GC status: %w", err)
		}

		fmt.Printf("\rProgress: %d%%", status.Progress)

		if !status.Running {
			fmt.Println()
			if status.Error != "" {
				return fmt.Errorf("GC failed: %s", status.Error)
			}
			fmt.Printf("GC completed. Collected: %d objects", status.Collected)
			if status.FreedMB > 0 {
				fmt.Printf(" (%.2f MB freed)", status.FreedMB)
			}
			fmt.Println()
			return nil
		}
		time.Sleep(500 * time.Millisecond)
	}
}

// exportStartResponse is the response body for POST /admin/v1/export.
type exportStartResponse struct {
	Started   bool   `json:"started"`
	Operation string `json:"operation"`
	Message   string `json:"message"`
}

// exportStatusResponse is the response body for GET /admin/v1/export/status.
type exportStatusResponse struct {
	Running   bool   `json:"running"`
	Operation string `json:"operation,omitempty"`
	Progress  int    `json:"progress"`
	StartedAt string `json:"started_at,omitempty"`
	DoneAt    string `json:"done_at,omitempty"`
	Error     string `json:"error,omitempty"`
}

func runAdminExport(_ *cobra.Command, args []string) error {
	outputDir := args[0]

	body := map[string]string{"destination": outputDir}
	result, err := adminJSON[exportStartResponse]("POST", "/admin/v1/export", body)
	if err != nil {
		return err
	}

	fmt.Printf("Export started: %s\n", result.Message)
	return pollExportStatus("export")
}

func runAdminImport(_ *cobra.Command, args []string) error {
	inputDir := args[0]

	if _, err := os.Stat(inputDir); err != nil {
		return fmt.Errorf("input directory %s: %w", inputDir, err)
	}

	body := map[string]string{"source": inputDir}
	result, err := adminJSON[exportStartResponse]("POST", "/admin/v1/import", body)
	if err != nil {
		return err
	}

	fmt.Printf("Import started: %s\n", result.Message)
	return pollExportStatus("import")
}

func pollExportStatus(op string) error {
	fmt.Printf("Polling %s status...\n", op)
	for {
		status, err := adminJSON[exportStatusResponse]("GET", "/admin/v1/export/status", nil)
		if err != nil {
			return fmt.Errorf("poll %s status: %w", op, err)
		}

		if flagJSON {
			_ = json.NewEncoder(os.Stdout).Encode(status)
		} else {
			fmt.Printf("\r%s progress: %d%%", op, status.Progress)
		}

		if !status.Running {
			fmt.Println()
			if status.Error != "" {
				return fmt.Errorf("%s failed: %s", op, status.Error)
			}
			fmt.Printf("%s completed.\n", op)
			return nil
		}
		time.Sleep(500 * time.Millisecond)
	}
}

// writeAdminToken writes the admin bearer token to ~/.sangraha/admin-token.
// Called by sangraha init after authentication is set up.
func writeAdminToken(token string) error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("resolve home dir: %w", err)
	}
	dir := homeDir + "/.sangraha"
	if err = os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}
	tokenPath := dir + "/admin-token"
	return os.WriteFile(tokenPath, []byte(token), 0600)
}

// readBodyAll is a helper used in admin operations to drain a response body.
func readBodyAll(r io.Reader) string {
	b, _ := io.ReadAll(r)
	return string(b)
}
