package cli

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// adminHTTPClient returns an *http.Client configured per the insecure flag.
func adminHTTPClient() *http.Client {
	if flagInsecure {
		return &http.Client{
			Timeout: 30 * time.Second,
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, //nolint:gosec // G402: only set when --insecure explicitly requested by operator
			},
		}
	}
	return &http.Client{Timeout: 30 * time.Second}
}

// readAdminToken reads the stored admin bearer token from ~/.sangraha/admin-token.
func readAdminToken() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home dir: %w", err)
	}
	tokenPath := filepath.Join(homeDir, ".sangraha", "admin-token")
	b, err := os.ReadFile(tokenPath) //nolint:gosec // G304: path is operator home dir
	if err != nil {
		return "", fmt.Errorf("read admin token from %s: %w (run 'sangraha init' first)", tokenPath, err)
	}
	return strings.TrimSpace(string(b)), nil
}

// adminDo performs an HTTP request against the admin API.
// body may be nil or any JSON-serialisable value.
func adminDo(method, path string, body any) (*http.Response, error) {
	token, err := readAdminToken()
	if err != nil {
		return nil, err
	}

	var reqBody io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("marshal request body: %w", err)
		}
		reqBody = bytes.NewReader(data)
	}

	url := strings.TrimRight(flagAdminURL, "/") + path
	req, err := http.NewRequest(method, url, reqBody)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := adminHTTPClient().Do(req)
	if err != nil {
		return nil, fmt.Errorf("admin API request %s %s: %w", method, path, err)
	}
	return resp, nil
}

// adminJSON performs an admin API request and decodes the JSON response into T.
func adminJSON[T any](method, path string, body any) (T, error) {
	var zero T
	resp, err := adminDo(method, path, body)
	if err != nil {
		return zero, err
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode >= 400 {
		raw, _ := io.ReadAll(resp.Body)
		return zero, fmt.Errorf("admin API %s %s returned %d: %s", method, path, resp.StatusCode, strings.TrimSpace(string(raw)))
	}

	if err = json.NewDecoder(resp.Body).Decode(&zero); err != nil {
		return zero, fmt.Errorf("decode admin API response: %w", err)
	}
	return zero, nil
}
