package auth

import (
	"encoding/json"
	"strings"
)

// Action constants for S3 IAM policy evaluation.
const (
	ActionPutObject             = "s3:PutObject"
	ActionGetObject             = "s3:GetObject"
	ActionDeleteObject          = "s3:DeleteObject"
	ActionHeadObject            = "s3:GetObject" // HeadObject uses s3:GetObject
	ActionCopyObject            = "s3:PutObject" // CopyObject requires PutObject on dst
	ActionListBucket            = "s3:ListBucket"
	ActionCreateBucket          = "s3:CreateBucket"
	ActionDeleteBucket          = "s3:DeleteBucket"
	ActionListAllMyBuckets      = "s3:ListAllMyBuckets"
	ActionPutBucketPolicy       = "s3:PutBucketPolicy"
	ActionGetBucketPolicy       = "s3:GetBucketPolicy"
	ActionDeleteBucketPolicy    = "s3:DeleteBucketPolicy"
	ActionAbortMultipartUpload  = "s3:AbortMultipartUpload"
	ActionListMultipartUploads  = "s3:ListBucketMultipartUploads"
	ActionGetBucketVersioning   = "s3:GetBucketVersioning"
	ActionPutBucketVersioning   = "s3:PutBucketVersioning"
	ActionGetObjectTagging      = "s3:GetObjectTagging"
	ActionPutObjectTagging      = "s3:PutObjectTagging"
	ActionDeleteObjectTagging   = "s3:DeleteObjectTagging"
	ActionGetBucketTagging      = "s3:GetBucketTagging"
	ActionPutBucketTagging      = "s3:PutBucketTagging"
	ActionDeleteBucketTagging   = "s3:DeleteBucketTagging"
	ActionGetBucketCORS         = "s3:GetBucketCORS"
	ActionPutBucketCORS         = "s3:PutBucketCORS"
	ActionDeleteBucketCORS      = "s3:DeleteBucketCORS"
	ActionGetBucketLifecycle    = "s3:GetLifecycleConfiguration"
	ActionPutBucketLifecycle    = "s3:PutLifecycleConfiguration"
	ActionDeleteBucketLifecycle = "s3:DeleteLifecycleConfiguration"
	ActionGetBucketACL          = "s3:GetBucketAcl"
	ActionPutBucketACL          = "s3:PutBucketAcl"
	ActionGetObjectACL          = "s3:GetObjectAcl"
	ActionPutObjectACL          = "s3:PutObjectAcl"
)

// Policy is a subset of the AWS IAM policy JSON document format.
type Policy struct {
	Version    string      `json:"Version"`
	Statements []Statement `json:"Statement"`
}

// Statement is a single IAM policy statement.
type Statement struct {
	Sid       string      `json:"Sid,omitempty"`
	Effect    string      `json:"Effect"` // "Allow" | "Deny"
	Principal interface{} `json:"Principal,omitempty"`
	Action    interface{} `json:"Action"`   // string or []string
	Resource  interface{} `json:"Resource"` // string or []string
	Condition interface{} `json:"Condition,omitempty"`
}

// ParsePolicy parses a bucket policy JSON string.
func ParsePolicy(policyJSON string) (*Policy, error) {
	if policyJSON == "" {
		return nil, nil
	}
	var p Policy
	if err := json.Unmarshal([]byte(policyJSON), &p); err != nil {
		return nil, err
	}
	return &p, nil
}

// EvalPolicy evaluates a bucket policy for the given subject, action, and resource.
// Returns allow=true when an Allow statement matches; deny=true when a Deny matches.
// Explicit deny always wins over allow.
func EvalPolicy(p *Policy, principal, action, resource string) (allow, deny bool) {
	if p == nil {
		return false, false
	}
	for _, stmt := range p.Statements {
		if !matchesPrincipal(stmt, principal) {
			continue
		}
		if !matchesActions(stmt, action) {
			continue
		}
		if !matchesResources(stmt, resource) {
			continue
		}
		if stmt.Effect == "Deny" {
			return false, true
		}
		if stmt.Effect == "Allow" {
			allow = true
		}
	}
	return allow, false
}

// IsAllowed performs IAM + root privilege evaluation.
// Root users are always allowed. For non-root users:
//   - If no bucket policy exists (policyJSON==""), allow all authenticated operations.
//   - If a bucket policy exists, evaluate it; default is deny.
func IsAllowed(isRoot bool, action, principal, bucket, key, policyJSON string) bool {
	if isRoot {
		return true
	}
	p, err := ParsePolicy(policyJSON)
	if err != nil {
		return false
	}
	if p == nil {
		// No bucket policy — allow all authenticated operations (Phase 2 default).
		return true
	}
	resource := "arn:aws:s3:::" + bucket
	if key != "" {
		resource += "/" + key
	}
	allow, deny := EvalPolicy(p, principal, action, resource)
	if deny {
		return false
	}
	return allow
}

func matchesPrincipal(stmt Statement, principal string) bool {
	if stmt.Principal == nil {
		return true
	}
	switch v := stmt.Principal.(type) {
	case string:
		return v == "*" || v == principal
	case map[string]interface{}:
		return matchesPrincipalMap(v, principal)
	case []interface{}:
		return matchesPrincipalSlice(v, principal)
	}
	return false
}

func matchesPrincipalMap(m map[string]interface{}, principal string) bool {
	for _, vals := range m {
		switch vv := vals.(type) {
		case string:
			if vv == "*" || vv == principal {
				return true
			}
		case []interface{}:
			if matchesPrincipalSlice(vv, principal) {
				return true
			}
		}
	}
	return false
}

func matchesPrincipalSlice(items []interface{}, principal string) bool {
	for _, item := range items {
		if s, ok := item.(string); ok && (s == "*" || s == principal) {
			return true
		}
	}
	return false
}

func matchesActions(stmt Statement, action string) bool {
	return matchesStringOrSlice(stmt.Action, action)
}

func matchesResources(stmt Statement, resource string) bool {
	return matchesStringOrSlice(stmt.Resource, resource)
}

func matchesStringOrSlice(val interface{}, target string) bool {
	switch v := val.(type) {
	case string:
		return globMatch(v, target)
	case []interface{}:
		for _, item := range v {
			if s, ok := item.(string); ok && globMatch(s, target) {
				return true
			}
		}
	}
	return false
}

// globMatch matches a pattern containing '*' and '?' wildcards against a target.
func globMatch(pattern, target string) bool {
	if pattern == "*" {
		return true
	}
	return globMatchRecursive(pattern, target)
}

func globMatchRecursive(pattern, target string) bool {
	for len(pattern) > 0 {
		switch pattern[0] {
		case '*':
			for len(pattern) > 0 && pattern[0] == '*' {
				pattern = pattern[1:]
			}
			if len(pattern) == 0 {
				return true
			}
			for i := 0; i <= len(target); i++ {
				if globMatchRecursive(pattern, target[i:]) {
					return true
				}
			}
			return false
		case '?':
			if len(target) == 0 {
				return false
			}
			pattern = pattern[1:]
			target = target[1:]
		default:
			if len(target) == 0 || !strings.EqualFold(pattern[:1], target[:1]) {
				return false
			}
			pattern = pattern[1:]
			target = target[1:]
		}
	}
	return len(target) == 0
}
