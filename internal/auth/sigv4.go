package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"
)

// SigV4 implements AWS Signature Version 4 request verification.
// Reference: https://docs.aws.amazon.com/general/latest/gr/sigv4_signing.html

const (
	// authHeaderPrefix is the prefix of the Authorization header value.
	authHeaderPrefix = "AWS4-HMAC-SHA256"
	// timestampFormat is the ISO 8601 format used in X-Amz-Date.
	timestampFormat = "20060102T150405Z"
	// dateFormat is the short date format used in credential scope.
	dateFormat = "20060102"
	// presignExpiry is the maximum allowed presigned URL expiry in seconds.
	presignExpiry = 7 * 24 * 3600 // 7 days, matching S3 spec
)

// VerifiedIdentity carries the authenticated identity extracted from a request.
type VerifiedIdentity struct {
	AccessKey string
	Owner     string
	IsRoot    bool
}

// VerifyRequest authenticates an HTTP request using SigV4. It returns the
// verified identity or an error describing the authentication failure.
func VerifyRequest(r *http.Request, secretKey string, now time.Time) error {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return fmt.Errorf("sigv4: missing Authorization header")
	}

	// Parse "AWS4-HMAC-SHA256 Credential=…, SignedHeaders=…, Signature=…"
	if !strings.HasPrefix(authHeader, authHeaderPrefix+" ") {
		return fmt.Errorf("sigv4: unsupported authorization scheme")
	}
	parts := parseAuthHeader(authHeader[len(authHeaderPrefix)+1:])

	credential := parts["Credential"]
	signedHeaders := parts["SignedHeaders"]
	providedSig := parts["Signature"]
	if credential == "" || signedHeaders == "" || providedSig == "" {
		return fmt.Errorf("sigv4: malformed Authorization header")
	}

	// Parse credential: <accessKey>/<date>/<region>/<service>/aws4_request
	credParts := strings.SplitN(credential, "/", 5)
	if len(credParts) != 5 || credParts[4] != "aws4_request" {
		return fmt.Errorf("sigv4: malformed credential scope")
	}
	dateStr := credParts[1]
	region := credParts[2]
	service := credParts[3]

	// Validate timestamp.
	amzDate := r.Header.Get("X-Amz-Date")
	if amzDate == "" {
		return fmt.Errorf("sigv4: missing X-Amz-Date header")
	}
	reqTime, err := time.Parse(timestampFormat, amzDate)
	if err != nil {
		return fmt.Errorf("sigv4: invalid X-Amz-Date %q: %w", amzDate, err)
	}
	// Allow 15-minute clock skew (S3 standard).
	if diff := now.Sub(reqTime); diff > 15*time.Minute || diff < -15*time.Minute {
		return fmt.Errorf("sigv4: request timestamp %s is too far from server time", amzDate)
	}
	if reqTime.Format(dateFormat) != dateStr {
		return fmt.Errorf("sigv4: credential date %s does not match X-Amz-Date %s", dateStr, amzDate)
	}

	// Canonical request.
	canonReq := canonicalRequest(r, strings.Split(signedHeaders, ";"))

	// String to sign.
	credScope := dateStr + "/" + region + "/" + service + "/aws4_request"
	stringToSign := authHeaderPrefix + "\n" +
		amzDate + "\n" +
		credScope + "\n" +
		hashSHA256(canonReq)

	// Signing key.
	signingKey := deriveSigningKey(secretKey, dateStr, region, service)

	// Expected signature.
	expectedSig := hex.EncodeToString(hmacSHA256(signingKey, []byte(stringToSign)))

	if !hmac.Equal([]byte(providedSig), []byte(expectedSig)) {
		return fmt.Errorf("sigv4: signature mismatch")
	}
	return nil
}

// ExtractAccessKey parses the access key from the Authorization header without
// verifying the signature. Returns "" if the header is absent or malformed.
func ExtractAccessKey(r *http.Request) string {
	authHeader := r.Header.Get("Authorization")
	if !strings.HasPrefix(authHeader, authHeaderPrefix+" ") {
		return ""
	}
	parts := parseAuthHeader(authHeader[len(authHeaderPrefix)+1:])
	cred := parts["Credential"]
	if cred == "" {
		return ""
	}
	return strings.SplitN(cred, "/", 2)[0]
}

// deriveSigningKey computes the HMAC-SHA256 signing key for a given date,
// region, and service using the AWS key derivation algorithm.
func deriveSigningKey(secretKey, date, region, service string) []byte {
	kDate := hmacSHA256([]byte("AWS4"+secretKey), []byte(date))
	kRegion := hmacSHA256(kDate, []byte(region))
	kService := hmacSHA256(kRegion, []byte(service))
	kSigning := hmacSHA256(kService, []byte("aws4_request"))
	return kSigning
}

// canonicalRequest builds the canonical request string as defined by the
// SigV4 specification.
func canonicalRequest(r *http.Request, signedHeaderNames []string) string {
	method := r.Method
	uri := canonicalURI(r.URL.Path)
	query := canonicalQueryString(r.URL.Query())
	headers, signedHeaderStr := canonicalHeaders(r, signedHeaderNames)
	payloadHash := r.Header.Get("X-Amz-Content-Sha256")
	if payloadHash == "" {
		payloadHash = "UNSIGNED-PAYLOAD"
	}
	return strings.Join([]string{method, uri, query, headers, signedHeaderStr, payloadHash}, "\n")
}

func canonicalURI(rawPath string) string {
	if rawPath == "" {
		return "/"
	}
	// URI-encode each path segment but preserve the slash separators.
	segments := strings.Split(rawPath, "/")
	for i, seg := range segments {
		segments[i] = uriEncode(seg, false)
	}
	return strings.Join(segments, "/")
}

func canonicalQueryString(params map[string][]string) string {
	if len(params) == 0 {
		return ""
	}
	pairs := make([]string, 0, len(params))
	for k, vals := range params {
		for _, v := range vals {
			pairs = append(pairs, uriEncode(k, true)+"="+uriEncode(v, true))
		}
	}
	sort.Strings(pairs)
	return strings.Join(pairs, "&")
}

func canonicalHeaders(r *http.Request, signedHeaderNames []string) (headers, signedStr string) {
	var sb strings.Builder
	sorted := make([]string, len(signedHeaderNames))
	copy(sorted, signedHeaderNames)
	sort.Strings(sorted)
	for _, name := range sorted {
		val := strings.TrimSpace(r.Header.Get(name))
		sb.WriteString(strings.ToLower(name) + ":" + val + "\n")
	}
	return sb.String(), strings.Join(sorted, ";")
}

func parseAuthHeader(s string) map[string]string {
	out := map[string]string{}
	for _, part := range strings.Split(s, ", ") {
		kv := strings.SplitN(strings.TrimSpace(part), "=", 2)
		if len(kv) == 2 {
			out[strings.TrimSpace(kv[0])] = strings.TrimSpace(kv[1])
		}
	}
	return out
}

func hmacSHA256(key, data []byte) []byte {
	h := hmac.New(sha256.New, key)
	h.Write(data)
	return h.Sum(nil)
}

func hashSHA256(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}

// uriEncode percent-encodes s. If all is true every character except
// unreserved characters is encoded; otherwise forward slashes are preserved.
func uriEncode(s string, all bool) string {
	var sb strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		if isUnreserved(c) || (!all && c == '/') {
			sb.WriteByte(c)
		} else {
			fmt.Fprintf(&sb, "%%%02X", c)
		}
	}
	return sb.String()
}

func isUnreserved(c byte) bool {
	return (c >= 'A' && c <= 'Z') ||
		(c >= 'a' && c <= 'z') ||
		(c >= '0' && c <= '9') ||
		c == '-' || c == '_' || c == '.' || c == '~'
}
