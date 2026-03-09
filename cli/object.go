package cli

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/madhavkobal/sangraha/pkg/s3types"
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
	RunE:  runObjectPut,
}

var objectGetCmd = &cobra.Command{
	Use:   "get <bucket> <key>",
	Short: "Download an object",
	Args:  cobra.ExactArgs(2),
	RunE:  runObjectGet,
}

var objectDeleteCmd = &cobra.Command{
	Use:   "delete <bucket> <key>",
	Short: "Delete an object",
	Args:  cobra.ExactArgs(2),
	RunE:  runObjectDelete,
}

var objectListCmd = &cobra.Command{
	Use:   "list <bucket>",
	Short: "List objects in a bucket",
	Args:  cobra.ExactArgs(1),
	RunE:  runObjectList,
}

var objectCpCmd = &cobra.Command{
	Use:   "cp <src> <dst>",
	Short: "Copy an object (s3://bucket/key URIs)",
	Args:  cobra.ExactArgs(2),
	RunE:  runObjectCp,
}

var objectMvCmd = &cobra.Command{
	Use:   "mv <src> <dst>",
	Short: "Move an object (s3://bucket/key URIs)",
	Args:  cobra.ExactArgs(2),
	RunE:  runObjectMv,
}

var objectPresignCmd = &cobra.Command{
	Use:   "presign <bucket> <key>",
	Short: "Generate a presigned URL for an object",
	Args:  cobra.ExactArgs(2),
	RunE:  runObjectPresign,
}

func init() {
	objectGetCmd.Flags().StringP("output", "o", "", "write output to file instead of stdout")
	objectPutCmd.Flags().String("content-type", "", "object content type")
	objectListCmd.Flags().String("prefix", "", "filter by prefix")
	objectListCmd.Flags().String("delimiter", "", "grouping delimiter")
	objectPresignCmd.Flags().Int("expires", 3600, "expiry in seconds (max 604800 / 7 days)")
	objectPresignCmd.Flags().String("method", "GET", "HTTP method: GET or PUT")
	objectDeleteCmd.Flags().Bool("force", false, "skip confirmation prompt")
	objectCmd.AddCommand(objectPutCmd, objectGetCmd, objectDeleteCmd, objectListCmd, objectCpCmd, objectMvCmd, objectPresignCmd)
}

func runObjectPut(cmd *cobra.Command, args []string) error {
	bucket, key, filePath := args[0], args[1], args[2]

	ctFlag, _ := cmd.Flags().GetString("content-type")

	var (
		body    io.Reader
		bodyLen int64
	)

	if filePath == "-" {
		body = os.Stdin
		bodyLen = -1
	} else {
		f, err := os.Open(filePath) //nolint:gosec // G304: operator-provided path
		if err != nil {
			return fmt.Errorf("open file %s: %w", filePath, err)
		}
		defer f.Close() //nolint:errcheck

		info, err := f.Stat()
		if err != nil {
			return fmt.Errorf("stat file %s: %w", filePath, err)
		}
		body = f
		bodyLen = info.Size()
	}

	ct := ctFlag
	if ct == "" {
		ct = detectContentType(filePath)
	}

	headers := map[string]string{}
	if ct != "" {
		headers["Content-Type"] = ct
	}

	resp, err := s3Do("PUT", bucket, key, nil, headers, body, bodyLen)
	if err != nil {
		return err
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode >= 400 {
		return s3ResponseError(resp)
	}

	etag := resp.Header.Get("ETag")
	if flagJSON {
		return json.NewEncoder(os.Stdout).Encode(map[string]string{
			"bucket": bucket, "key": key, "etag": etag,
		})
	}
	fmt.Printf("Uploaded s3://%s/%s (ETag: %s)\n", bucket, key, etag)
	return nil
}

func runObjectGet(cmd *cobra.Command, args []string) error {
	bucket, key := args[0], args[1]
	outputFile, _ := cmd.Flags().GetString("output")

	resp, err := s3Do("GET", bucket, key, nil, nil, nil, 0)
	if err != nil {
		return err
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode >= 400 {
		return s3ResponseError(resp)
	}

	var dst io.Writer
	if outputFile != "" {
		f, err := os.Create(outputFile) //nolint:gosec // G304: operator-provided path
		if err != nil {
			return fmt.Errorf("create output file %s: %w", outputFile, err)
		}
		defer f.Close() //nolint:errcheck
		dst = f
	} else {
		dst = os.Stdout
	}

	if _, err = io.Copy(dst, resp.Body); err != nil {
		return fmt.Errorf("write object data: %w", err)
	}
	return nil
}

func runObjectDelete(cmd *cobra.Command, args []string) error {
	bucket, key := args[0], args[1]

	force, _ := cmd.Flags().GetBool("force")
	if !force {
		if !confirmAction(fmt.Sprintf("delete s3://%s/%s", bucket, key)) {
			return nil
		}
	}

	resp, err := s3Do("DELETE", bucket, key, nil, nil, nil, 0)
	if err != nil {
		return err
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode >= 400 {
		return s3ResponseError(resp)
	}
	fmt.Printf("Deleted s3://%s/%s\n", bucket, key)
	return nil
}

func runObjectList(cmd *cobra.Command, args []string) error {
	bucket := args[0]
	prefix, _ := cmd.Flags().GetString("prefix")
	delimiter, _ := cmd.Flags().GetString("delimiter")

	q := url.Values{"list-type": []string{"2"}}
	if prefix != "" {
		q.Set("prefix", prefix)
	}
	if delimiter != "" {
		q.Set("delimiter", delimiter)
	}

	resp, err := s3Do("GET", bucket, "", q, nil, nil, 0)
	if err != nil {
		return err
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode >= 400 {
		return s3ResponseError(resp)
	}

	var result s3types.ListBucketResult
	if err = xml.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("decode list objects response: %w", err)
	}

	if flagJSON {
		return json.NewEncoder(os.Stdout).Encode(result.Contents)
	}

	tw := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	if _, err = fmt.Fprintln(tw, "KEY\tSIZE\tLAST MODIFIED\tETAG"); err != nil {
		return err
	}
	for _, obj := range result.Contents {
		if _, err = fmt.Fprintf(tw, "%s\t%d\t%s\t%s\n",
			obj.Key, obj.Size,
			obj.LastModified.Format("2006-01-02 15:04:05"),
			obj.ETag); err != nil {
			return err
		}
	}
	return tw.Flush()
}

// parseS3URI parses an s3://bucket/key URI into bucket and key components.
func parseS3URI(uri string) (bucket, key string, err error) {
	if !strings.HasPrefix(uri, "s3://") {
		return "", "", fmt.Errorf("invalid S3 URI %q: must start with s3://", uri)
	}
	rest := strings.TrimPrefix(uri, "s3://")
	parts := strings.SplitN(rest, "/", 2)
	if len(parts) < 2 || parts[0] == "" || parts[1] == "" {
		return "", "", fmt.Errorf("invalid S3 URI %q: must be s3://bucket/key", uri)
	}
	return parts[0], parts[1], nil
}

func runObjectCp(_ *cobra.Command, args []string) error {
	srcURI, dstURI := args[0], args[1]

	srcBucket, srcKey, err := parseS3URI(srcURI)
	if err != nil {
		return err
	}
	dstBucket, dstKey, err := parseS3URI(dstURI)
	if err != nil {
		return err
	}

	return doCopyObject(srcBucket, srcKey, dstBucket, dstKey)
}

func runObjectMv(_ *cobra.Command, args []string) error {
	srcURI, dstURI := args[0], args[1]

	srcBucket, srcKey, err := parseS3URI(srcURI)
	if err != nil {
		return err
	}
	dstBucket, dstKey, err := parseS3URI(dstURI)
	if err != nil {
		return err
	}

	if err = doCopyObject(srcBucket, srcKey, dstBucket, dstKey); err != nil {
		return err
	}

	// Delete the source after successful copy.
	resp, err := s3Do("DELETE", srcBucket, srcKey, nil, nil, nil, 0)
	if err != nil {
		return fmt.Errorf("delete source after copy: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode >= 400 {
		return fmt.Errorf("delete source after copy: %w", s3ResponseError(resp))
	}
	fmt.Printf("Moved s3://%s/%s -> s3://%s/%s\n", srcBucket, srcKey, dstBucket, dstKey)
	return nil
}

func doCopyObject(srcBucket, srcKey, dstBucket, dstKey string) error {
	copySrc := "/" + srcBucket + "/" + srcKey
	headers := map[string]string{
		"x-amz-copy-source": copySrc,
	}

	resp, err := s3Do("PUT", dstBucket, dstKey, nil, headers, nil, 0)
	if err != nil {
		return err
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode >= 400 {
		return s3ResponseError(resp)
	}
	fmt.Printf("Copied s3://%s/%s -> s3://%s/%s\n", srcBucket, srcKey, dstBucket, dstKey)
	return nil
}

// presignResponseBody mirrors the admin API presign response.
type presignResponseBody struct {
	URL       string `json:"url"`
	ExpiresAt string `json:"expires_at"`
}

func runObjectPresign(cmd *cobra.Command, args []string) error {
	bucket, key := args[0], args[1]
	expires, _ := cmd.Flags().GetInt("expires")
	method, _ := cmd.Flags().GetString("method")

	method = strings.ToUpper(method)
	if method != "GET" && method != "PUT" {
		return fmt.Errorf("method must be GET or PUT")
	}

	body := map[string]interface{}{
		"bucket":     bucket,
		"key":        key,
		"method":     method,
		"expires_in": expires,
	}

	result, err := adminJSON[presignResponseBody]("POST", "/admin/v1/presign", body)
	if err != nil {
		return err
	}

	if flagJSON {
		return json.NewEncoder(os.Stdout).Encode(result)
	}
	fmt.Printf("Presigned URL (%s, expires %s):\n%s\n", method, result.ExpiresAt, result.URL)
	return nil
}

// detectContentType guesses the MIME type from the file extension.
func detectContentType(filePath string) string {
	if filePath == "-" {
		return "application/octet-stream"
	}
	ext := strings.ToLower(filepath.Ext(filePath))
	if ext == "" {
		return "application/octet-stream"
	}
	ct := mime.TypeByExtension(ext)
	if ct == "" {
		return "application/octet-stream"
	}
	return ct
}

// s3XMLError is the S3 XML error response structure.
type s3XMLError struct {
	Code      string `xml:"Code"`
	Message   string `xml:"Message"`
	RequestID string `xml:"RequestId"`
	Resource  string `xml:"Resource"`
}

// s3ResponseError reads an *http.Response body as an S3 XML error and returns a Go error.
func s3ResponseError(resp *http.Response) error {
	raw, _ := io.ReadAll(resp.Body)

	var s3err s3XMLError
	if xmlErr := xml.Unmarshal(raw, &s3err); xmlErr == nil && s3err.Code != "" {
		return fmt.Errorf("S3 error %s: %s", s3err.Code, s3err.Message)
	}
	return fmt.Errorf("S3 request failed with HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(raw)))
}

// newBytesReader creates a *bytes.Reader from a byte slice.
func newBytesReader(b []byte) io.Reader {
	return bytes.NewReader(b)
}
