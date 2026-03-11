package cli

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"net/url"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/madhavkobal/sangraha/pkg/s3types"
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
	RunE:  runBucketCreate,
}

var bucketDeleteCmd = &cobra.Command{
	Use:   "delete <name>",
	Short: "Delete a bucket",
	Args:  cobra.ExactArgs(1),
	RunE:  runBucketDelete,
}

var bucketListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all buckets",
	RunE:  runBucketList,
}

var bucketVersioningCmd = &cobra.Command{
	Use:   "versioning <name> <enable|suspend|disable>",
	Short: "Set the versioning state of a bucket",
	Args:  cobra.ExactArgs(2),
	RunE:  runBucketVersioning,
}

var bucketPolicyCmd = &cobra.Command{
	Use:   "policy",
	Short: "Manage bucket policies",
}

var bucketPolicyGetCmd = &cobra.Command{
	Use:   "get <name>",
	Short: "Get the bucket policy",
	Args:  cobra.ExactArgs(1),
	RunE:  runBucketPolicyGet,
}

var bucketPolicySetCmd = &cobra.Command{
	Use:   "set <name> <file>",
	Short: "Set the bucket policy from a JSON file",
	Args:  cobra.ExactArgs(2),
	RunE:  runBucketPolicySet,
}

func init() {
	bucketDeleteCmd.Flags().Bool("force", false, "skip confirmation prompt")
	bucketPolicyCmd.AddCommand(bucketPolicyGetCmd, bucketPolicySetCmd)
	bucketCmd.AddCommand(bucketCreateCmd, bucketDeleteCmd, bucketListCmd, bucketVersioningCmd, bucketPolicyCmd)
}

func runBucketCreate(_ *cobra.Command, args []string) error {
	name := args[0]
	resp, err := s3Do("PUT", name, "", nil, nil, nil, 0)
	if err != nil {
		return err
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode >= 400 {
		return s3ResponseError(resp)
	}
	fmt.Printf("Bucket %q created.\n", name)
	return nil
}

func runBucketDelete(cmd *cobra.Command, args []string) error {
	name := args[0]

	force, _ := cmd.Flags().GetBool("force")
	if !force {
		if !confirmAction(fmt.Sprintf("delete bucket %q", name)) {
			return nil
		}
	}

	resp, err := s3Do("DELETE", name, "", nil, nil, nil, 0)
	if err != nil {
		return err
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode >= 400 {
		return s3ResponseError(resp)
	}
	fmt.Printf("Bucket %q deleted.\n", name)
	return nil
}

func runBucketList(_ *cobra.Command, _ []string) error {
	resp, err := s3Do("GET", "", "", nil, nil, nil, 0)
	if err != nil {
		return err
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode >= 400 {
		return s3ResponseError(resp)
	}

	var result s3types.ListAllMyBucketsResult
	if err = xml.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("decode list buckets response: %w", err)
	}

	if flagJSON {
		type bucketJSON struct {
			Name    string `json:"name"`
			Created string `json:"created"`
		}
		out := make([]bucketJSON, len(result.Buckets))
		for i, b := range result.Buckets {
			out[i] = bucketJSON{Name: b.Name, Created: b.CreationDate.String()}
		}
		return json.NewEncoder(os.Stdout).Encode(out)
	}

	tw := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	if _, err = fmt.Fprintln(tw, "NAME\tCREATED"); err != nil {
		return err
	}
	for _, b := range result.Buckets {
		if _, err = fmt.Fprintf(tw, "%s\t%s\n", b.Name, b.CreationDate.Format("2006-01-02 15:04:05")); err != nil {
			return err
		}
	}
	return tw.Flush()
}

// versioningRequest is the S3 VersioningConfiguration XML body.
type versioningRequest struct {
	XMLName xml.Name `xml:"VersioningConfiguration"`
	Xmlns   string   `xml:"xmlns,attr"`
	Status  string   `xml:"Status"`
}

func runBucketVersioning(_ *cobra.Command, args []string) error {
	name, state := args[0], args[1]

	var s3Status string
	switch state {
	case "enable":
		s3Status = "Enabled"
	case "suspend":
		s3Status = "Suspended"
	case "disable":
		// S3 has no "disabled"; use suspended to turn it off.
		s3Status = "Suspended"
	default:
		return fmt.Errorf("versioning state must be one of: enable, suspend, disable")
	}

	xmlBody, err := xml.Marshal(versioningRequest{
		Xmlns:  "http://s3.amazonaws.com/doc/2006-03-01/",
		Status: s3Status,
	})
	if err != nil {
		return fmt.Errorf("marshal versioning config: %w", err)
	}

	xmlBody = append([]byte(xml.Header), xmlBody...)
	body := newBytesReader(xmlBody)
	q := url.Values{"versioning": []string{""}}

	resp, err := s3Do("PUT", name, "", q, map[string]string{"Content-Type": "application/xml"}, body, int64(len(xmlBody)))
	if err != nil {
		return err
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode >= 400 {
		return s3ResponseError(resp)
	}
	fmt.Printf("Bucket %q versioning set to %q.\n", name, state)
	return nil
}

func runBucketPolicyGet(_ *cobra.Command, args []string) error {
	name := args[0]
	q := url.Values{"policy": []string{""}}

	resp, err := s3Do("GET", name, "", q, nil, nil, 0)
	if err != nil {
		return err
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode == 404 {
		fmt.Println("No policy set for this bucket.")
		return nil
	}
	if resp.StatusCode >= 400 {
		return s3ResponseError(resp)
	}

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read policy response: %w", err)
	}
	fmt.Println(string(raw))
	return nil
}

func runBucketPolicySet(_ *cobra.Command, args []string) error {
	name, file := args[0], args[1]

	data, err := os.ReadFile(file) //nolint:gosec // G304: path is operator-provided
	if err != nil {
		return fmt.Errorf("read policy file %s: %w", file, err)
	}

	// Validate that the file is valid JSON.
	var check interface{}
	if err = json.Unmarshal(data, &check); err != nil {
		return fmt.Errorf("policy file is not valid JSON: %w", err)
	}

	q := url.Values{"policy": []string{""}}
	body := newBytesReader(data)

	resp, err := s3Do("PUT", name, "", q, map[string]string{"Content-Type": "application/json"}, body, int64(len(data)))
	if err != nil {
		return err
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode >= 400 {
		return s3ResponseError(resp)
	}
	fmt.Printf("Policy for bucket %q updated.\n", name)
	return nil
}
