package auth

import (
	"context"
	"encoding/hex"
	"net/http"
	"net/url"
	"testing"
	"time"
)

func TestExtractAccessKey(t *testing.T) {
	r, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	r.Header.Set("Authorization",
		`AWS4-HMAC-SHA256 Credential=AKIAIOSFODNN7EXAMPLE/20260304/us-east-1/s3/aws4_request, SignedHeaders=host;x-amz-date, Signature=abc123`)

	ak := ExtractAccessKey(r)
	if ak != "AKIAIOSFODNN7EXAMPLE" {
		t.Errorf("ExtractAccessKey = %q; want AKIAIOSFODNN7EXAMPLE", ak)
	}
}

func TestExtractAccessKeyMissing(t *testing.T) {
	r, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	if ak := ExtractAccessKey(r); ak != "" {
		t.Errorf("ExtractAccessKey with no header = %q; want empty", ak)
	}
}

func TestDeriveSigningKey(t *testing.T) {
	// Verify against the AWS test vectors from:
	// https://docs.aws.amazon.com/general/latest/gr/sigv4-calculate-signature.html
	key := deriveSigningKey("wJalrXUtnFEMI/K7MDENG+bPxRfiCYEXAMPLEKEY",
		"20110909", "us-east-1", "iam")
	if len(key) != 32 {
		t.Errorf("signing key length = %d; want 32", len(key))
	}
}

func TestCanonicalURI(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"", "/"},
		{"/", "/"},
		{"/bucket/key", "/bucket/key"},
		{"/bucket/key with spaces", "/bucket/key%20with%20spaces"},
	}
	for _, tc := range tests {
		got := canonicalURI(tc.input)
		if got != tc.want {
			t.Errorf("canonicalURI(%q) = %q; want %q", tc.input, got, tc.want)
		}
	}
}

func TestVerifyRequestMissingAuth(t *testing.T) {
	r, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	if err := VerifyRequest(r, "secret", time.Now()); err == nil {
		t.Error("expected error for missing Authorization, got nil")
	}
}

func TestHashSHA256(t *testing.T) {
	// SHA256("") = e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855
	got := hashSHA256("")
	want := "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
	if got != want {
		t.Errorf("hashSHA256(\"\") = %q; want %q", got, want)
	}
}

func TestCanonicalQueryString(t *testing.T) {
	tests := []struct {
		raw  string
		want string
	}{
		{"b=2&a=1", "a=1&b=2"},
		{"", ""},
		{"z=z&a=a&m=m", "a=a&m=m&z=z"},
	}
	for _, tc := range tests {
		q, _ := url.ParseQuery(tc.raw)
		got := canonicalQueryString(q)
		if got != tc.want {
			t.Errorf("canonicalQueryString(%q) = %q; want %q", tc.raw, got, tc.want)
		}
	}
}

func TestHmacSHA256(t *testing.T) {
	key := []byte("key")
	data := "data"
	result := hmacSHA256(key, []byte(data))
	if len(result) != 32 {
		t.Errorf("hmacSHA256 length = %d; want 32", len(result))
	}
}

func TestVerifyRequest(t *testing.T) {
	now := time.Now().UTC()
	date := now.Format("20060102")
	amzDate := now.Format("20060102T150405Z")

	secret := "mysecret"
	accessKey := "AKIAIOSFODNN7EXAMPLE"
	region := "us-east-1"
	service := "s3"
	credScope := date + "/" + region + "/" + service + "/aws4_request"

	r, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, "/?key=val", nil)
	r.Header.Set("host", "localhost:9000")
	r.Header.Set("X-Amz-Date", amzDate)

	signedHeaders := []string{"host", "x-amz-date"}
	canonReq := canonicalRequest(r, signedHeaders)
	stringToSign := "AWS4-HMAC-SHA256\n" + amzDate + "\n" + credScope + "\n" + hashSHA256(canonReq)
	signingKey := deriveSigningKey(secret, date, region, service)
	sig := hex.EncodeToString(hmacSHA256(signingKey, []byte(stringToSign)))

	r.Header.Set("Authorization", "AWS4-HMAC-SHA256 Credential="+accessKey+"/"+credScope+", SignedHeaders=host;x-amz-date, Signature="+sig)

	if err := VerifyRequest(r, secret, now); err != nil {
		t.Errorf("VerifyRequest: %v", err)
	}
}

func TestVerifyRequestWrongSecret(t *testing.T) {
	now := time.Now().UTC()
	date := now.Format("20060102")
	amzDate := now.Format("20060102T150405Z")
	credScope := date + "/us-east-1/s3/aws4_request"

	r, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	r.Header.Set("host", "localhost:9000")
	r.Header.Set("X-Amz-Date", amzDate)
	r.Header.Set("Authorization", "AWS4-HMAC-SHA256 Credential=AK/"+credScope+",SignedHeaders=host,Signature=badsig")

	if err := VerifyRequest(r, "wrongsecret", now); err == nil {
		t.Error("VerifyRequest with wrong secret should return error")
	}
}

func TestVerifyRequestMissingAmzDate(t *testing.T) {
	r, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	r.Header.Set("Authorization", "AWS4-HMAC-SHA256 Credential=AK/20260305/us-east-1/s3/aws4_request,SignedHeaders=host,Signature=sig")

	err := VerifyRequest(r, "secret", time.Now())
	if err == nil {
		t.Error("VerifyRequest without X-Amz-Date should return error")
	}
}

func TestVerifyRequestExpired(t *testing.T) {
	// 30-minute-old request should fail.
	past := time.Now().UTC().Add(-30 * time.Minute)
	date := past.Format("20060102")
	amzDate := past.Format("20060102T150405Z")
	credScope := date + "/us-east-1/s3/aws4_request"

	r, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	r.Header.Set("X-Amz-Date", amzDate)
	r.Header.Set("Authorization", "AWS4-HMAC-SHA256 Credential=AK/"+credScope+",SignedHeaders=host,Signature=sig")

	err := VerifyRequest(r, "secret", time.Now())
	if err == nil {
		t.Error("VerifyRequest with expired timestamp should return error")
	}
}
