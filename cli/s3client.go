package cli

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
)

const (
	s3Region        = "us-east-1"
	s3Service       = "s3"
	s3TimestampFmt  = "20060102T150405Z"
	s3DateFmt       = "20060102"
	unsignedPayload = "UNSIGNED-PAYLOAD"
)

// s3HTTPClient returns an *http.Client for S3 API calls.
func s3HTTPClient() *http.Client {
	if flagInsecure {
		return &http.Client{
			Timeout: 120 * time.Second,
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, //nolint:gosec // G402: only set when --insecure explicitly requested by operator
			},
		}
	}
	return &http.Client{Timeout: 120 * time.Second}
}

// s3Do builds, signs, and executes an S3 API request.
// body may be nil for requests without a body.
func s3Do(method, bucket, key string, query url.Values, headers map[string]string, body io.Reader, bodyLen int64) (*http.Response, error) {
	if flagAccessKey == "" || flagSecretKey == "" {
		return nil, fmt.Errorf("access key and secret key are required (set --access-key / --secret-key or SANGRAHA_ACCESS_KEY / SANGRAHA_SECRET_KEY)")
	}

	rawPath := "/"
	if bucket != "" {
		rawPath += bucket
		if key != "" {
			rawPath += "/" + key
		}
	}

	base := strings.TrimRight(flagServer, "/")
	rawURL := base + rawPath
	if len(query) > 0 {
		rawURL += "?" + query.Encode()
	}

	req, err := http.NewRequest(method, rawURL, body)
	if err != nil {
		return nil, fmt.Errorf("build S3 request: %w", err)
	}

	now := time.Now().UTC()
	amzDate := now.Format(s3TimestampFmt)
	dateStr := now.Format(s3DateFmt)

	req.Header.Set("X-Amz-Date", amzDate)
	req.Header.Set("X-Amz-Content-Sha256", unsignedPayload)
	if bodyLen >= 0 {
		req.ContentLength = bodyLen
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	// Ensure Host is set for signing.
	parsedURL, _ := url.Parse(rawURL)
	req.Host = parsedURL.Host

	signS3Request(req, flagAccessKey, flagSecretKey, dateStr, amzDate)

	resp, err := s3HTTPClient().Do(req)
	if err != nil {
		return nil, fmt.Errorf("S3 request %s %s: %w", method, rawPath, err)
	}
	return resp, nil
}

// signS3Request adds the Authorization header to req using SigV4.
func signS3Request(req *http.Request, accessKey, secretKey, dateStr, amzDate string) {
	signedHeaderNames := []string{"host", "x-amz-content-sha256", "x-amz-date"}
	// Include any extra headers from the request that should be signed.
	for _, extra := range []string{"content-type", "x-amz-copy-source"} {
		if req.Header.Get(extra) != "" {
			signedHeaderNames = append(signedHeaderNames, extra)
		}
	}
	sort.Strings(signedHeaderNames)

	canonReq := buildCanonicalRequest(req, signedHeaderNames)
	credScope := dateStr + "/" + s3Region + "/" + s3Service + "/aws4_request"
	stringToSign := "AWS4-HMAC-SHA256\n" + amzDate + "\n" + credScope + "\n" + hexSHA256([]byte(canonReq))

	signingKey := deriveKey(secretKey, dateStr, s3Region, s3Service)
	sig := hex.EncodeToString(hmacBytes(signingKey, []byte(stringToSign)))

	authHeader := "AWS4-HMAC-SHA256 Credential=" + accessKey + "/" + credScope +
		", SignedHeaders=" + strings.Join(signedHeaderNames, ";") +
		", Signature=" + sig
	req.Header.Set("Authorization", authHeader)
}

func buildCanonicalRequest(req *http.Request, signedHeaderNames []string) string {
	method := req.Method

	rawPath := req.URL.EscapedPath()
	if rawPath == "" {
		rawPath = "/"
	}
	canonURI := encodeURIPath(rawPath)

	// Canonical query string: sorted key=value pairs.
	q := req.URL.Query()
	pairs := make([]string, 0, len(q))
	for k, vals := range q {
		for _, v := range vals {
			pairs = append(pairs, uriEncode(k)+"="+uriEncode(v))
		}
	}
	sort.Strings(pairs)
	canonQuery := strings.Join(pairs, "&")

	// Canonical headers.
	var hdrBuf strings.Builder
	for _, name := range signedHeaderNames {
		var val string
		if name == "host" {
			val = req.Host
			if val == "" {
				val = req.URL.Host
			}
		} else {
			val = strings.TrimSpace(req.Header.Get(name))
		}
		hdrBuf.WriteString(name + ":" + val + "\n")
	}
	canonHeaders := hdrBuf.String()
	signedHeaderStr := strings.Join(signedHeaderNames, ";")

	payloadHash := req.Header.Get("X-Amz-Content-Sha256")
	if payloadHash == "" {
		payloadHash = unsignedPayload
	}

	return strings.Join([]string{method, canonURI, canonQuery, canonHeaders, signedHeaderStr, payloadHash}, "\n")
}

func deriveKey(secretKey, date, region, service string) []byte {
	kDate := hmacBytes([]byte("AWS4"+secretKey), []byte(date))
	kRegion := hmacBytes(kDate, []byte(region))
	kService := hmacBytes(kRegion, []byte(service))
	return hmacBytes(kService, []byte("aws4_request"))
}

func hmacBytes(key, data []byte) []byte {
	h := hmac.New(sha256.New, key)
	h.Write(data)
	return h.Sum(nil)
}

func hexSHA256(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}

// encodeURIPath encodes a URI path — each segment is encoded but slashes preserved.
func encodeURIPath(path string) string {
	segments := strings.Split(path, "/")
	for i, seg := range segments {
		segments[i] = uriEncode(seg)
	}
	return strings.Join(segments, "/")
}

// uriEncode percent-encodes s, encoding everything except unreserved characters.
func uriEncode(s string) string {
	var sb strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		if isUnreservedByte(c) {
			sb.WriteByte(c)
		} else {
			fmt.Fprintf(&sb, "%%%02X", c)
		}
	}
	return sb.String()
}

func isUnreservedByte(c byte) bool {
	return (c >= 'A' && c <= 'Z') ||
		(c >= 'a' && c <= 'z') ||
		(c >= '0' && c <= '9') ||
		c == '-' || c == '_' || c == '.' || c == '~'
}
