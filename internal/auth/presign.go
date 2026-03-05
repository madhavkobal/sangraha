package auth

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// PresignedParams holds the parsed parameters of a presigned URL.
type PresignedParams struct {
	AccessKey  string
	Date       string
	Expires    int64
	Region     string
	Service    string
	Algorithm  string
	Signature  string
	SignedHdrs string
}

// GeneratePresignedURL creates an AWS SigV4-compatible presigned URL for a
// request. It returns the full URL with auth query parameters appended.
func GeneratePresignedURL(
	method, rawURL, bucket, key string,
	accessKey, secretKey string,
	region string,
	expires time.Duration,
	now time.Time,
) (string, error) {
	if expires <= 0 || expires > 7*24*time.Hour {
		return "", fmt.Errorf("presign: expires must be between 1s and 7 days")
	}

	date := now.UTC().Format(dateFormat)
	timestamp := now.UTC().Format(timestampFormat)
	expireSec := strconv.FormatInt(int64(expires.Seconds()), 10)

	credScope := date + "/" + region + "/s3/aws4_request"
	credential := accessKey + "/" + credScope

	signedHeaders := "host"

	// Build URL without signature first.
	path := "/" + bucket + "/" + key
	q := fmt.Sprintf(
		"X-Amz-Algorithm=%s&X-Amz-Credential=%s&X-Amz-Date=%s&X-Amz-Expires=%s&X-Amz-SignedHeaders=%s",
		authHeaderPrefix,
		uriEncode(credential, true),
		timestamp,
		expireSec,
		signedHeaders,
	)

	// Canonical request uses UNSIGNED-PAYLOAD for presigned URLs.
	host := extractHost(rawURL, bucket)
	canonReqStr := method + "\n" +
		canonicalURI(path) + "\n" +
		q + "\n" +
		"host:" + host + "\n" +
		"\n" +
		signedHeaders + "\n" +
		"UNSIGNED-PAYLOAD"

	stringToSign := authHeaderPrefix + "\n" +
		timestamp + "\n" +
		credScope + "\n" +
		hashSHA256(canonReqStr)

	signingKey := deriveSigningKey(secretKey, date, region, "s3")
	sig := fmt.Sprintf("%x", hmacSHA256(signingKey, []byte(stringToSign)))

	return rawURL + path + "?" + q + "&X-Amz-Signature=" + sig, nil
}

// VerifyPresignedURL verifies the SigV4 presigned URL signature on r.
func VerifyPresignedURL(r *http.Request, secretKey string, now time.Time) error {
	q := r.URL.Query()

	algorithm := q.Get("X-Amz-Algorithm")
	if algorithm == "" {
		algorithm = q.Get("x-amz-algorithm")
	}
	if algorithm != authHeaderPrefix {
		return fmt.Errorf("presign: unsupported algorithm %q", algorithm)
	}

	credential := q.Get("X-Amz-Credential")
	if credential == "" {
		credential = q.Get("x-amz-credential")
	}
	amzDate := q.Get("X-Amz-Date")
	if amzDate == "" {
		amzDate = q.Get("x-amz-date")
	}
	expiresStr := q.Get("X-Amz-Expires")
	if expiresStr == "" {
		expiresStr = q.Get("x-amz-expires")
	}
	signedHeaders := q.Get("X-Amz-SignedHeaders")
	if signedHeaders == "" {
		signedHeaders = q.Get("x-amz-signedheaders")
	}
	providedSig := q.Get("X-Amz-Signature")
	if providedSig == "" {
		providedSig = q.Get("x-amz-signature")
	}

	if credential == "" || amzDate == "" || expiresStr == "" || signedHeaders == "" || providedSig == "" {
		return fmt.Errorf("presign: missing required query parameters")
	}

	reqTime, err := time.Parse(timestampFormat, amzDate)
	if err != nil {
		return fmt.Errorf("presign: invalid X-Amz-Date: %w", err)
	}
	expSec, err := strconv.ParseInt(expiresStr, 10, 64)
	if err != nil {
		return fmt.Errorf("presign: invalid X-Amz-Expires: %w", err)
	}
	if now.After(reqTime.Add(time.Duration(expSec) * time.Second)) {
		return fmt.Errorf("presign: URL has expired")
	}

	// Parse credential scope.
	credParts := strings.SplitN(credential, "/", 5)
	if len(credParts) != 5 {
		return fmt.Errorf("presign: malformed credential")
	}
	dateStr := credParts[1]
	region := credParts[2]
	service := credParts[3]

	// Rebuild canonical query string (all params except signature).
	cleanQ := r.URL.Query()
	cleanQ.Del("X-Amz-Signature")
	cleanQ.Del("x-amz-signature")
	canonQ := canonicalQueryString(cleanQ)

	// Canonical headers.
	hdrNames := strings.Split(signedHeaders, ";")
	canonHdrs, _ := canonicalHeaders(r, hdrNames)

	canonReq := r.Method + "\n" +
		canonicalURI(r.URL.Path) + "\n" +
		canonQ + "\n" +
		canonHdrs + "\n" +
		signedHeaders + "\n" +
		"UNSIGNED-PAYLOAD"

	credScope := dateStr + "/" + region + "/" + service + "/aws4_request"
	stringToSign := authHeaderPrefix + "\n" +
		amzDate + "\n" +
		credScope + "\n" +
		hashSHA256(canonReq)

	signingKey := deriveSigningKey(secretKey, dateStr, region, service)
	expectedSig := fmt.Sprintf("%x", hmacSHA256(signingKey, []byte(stringToSign)))

	if providedSig != expectedSig {
		return fmt.Errorf("presign: signature mismatch")
	}
	return nil
}

// extractHost returns the host portion of a raw URL (without scheme).
func extractHost(rawURL, bucket string) string {
	rawURL = strings.TrimPrefix(rawURL, "https://")
	rawURL = strings.TrimPrefix(rawURL, "http://")
	// Remove path component.
	if idx := strings.Index(rawURL, "/"); idx >= 0 {
		rawURL = rawURL[:idx]
	}
	if rawURL == "" {
		return bucket + ".s3.amazonaws.com"
	}
	return rawURL
}
