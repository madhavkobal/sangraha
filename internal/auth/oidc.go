// Package auth provides authentication mechanisms including OIDC integration.
// OIDCProvider authenticates users against an OIDC IdP (Google, Keycloak, etc).
// When OIDC is enabled, the admin API accepts Bearer tokens issued by the IdP
// in addition to the local HMAC-signed tokens.
package auth

import (
	"context"
	"crypto/ecdsa"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"strings"
	"sync"
	"time"

	"golang.org/x/oauth2"
)

// OIDCConfig holds the configuration for an OIDC identity provider.
type OIDCConfig struct {
	// Enabled controls whether OIDC authentication is active.
	Enabled bool `yaml:"enabled" mapstructure:"enabled"`
	// IssuerURL is the base URL of the OIDC IdP (e.g. https://accounts.google.com).
	IssuerURL string `yaml:"issuer_url" mapstructure:"issuer_url"`
	// ClientID is the registered OAuth2 client identifier.
	ClientID string `yaml:"client_id" mapstructure:"client_id"`
	// ClientSecret must be set via the SANGRAHA_AUTH_OIDC_CLIENT_SECRET env var only.
	ClientSecret string `yaml:"-" mapstructure:"-"`
	// RedirectURL is the callback URL registered with the IdP.
	RedirectURL string `yaml:"redirect_url" mapstructure:"redirect_url"`
	// GroupsClaim is the JWT claim name containing group memberships (default "groups").
	GroupsClaim string `yaml:"groups_claim" mapstructure:"groups_claim"`
	// AdminGroups lists groups that get admin-level access.
	AdminGroups []string `yaml:"admin_groups" mapstructure:"admin_groups"`
}

// oidcDiscovery holds the subset of fields from the OIDC discovery document.
type oidcDiscovery struct {
	Issuer  string `json:"issuer"`
	JWKSURI string `json:"jwks_uri"`
	// AuthEndpoint is the authorization endpoint.
	AuthEndpoint  string `json:"authorization_endpoint"`
	TokenEndpoint string `json:"token_endpoint"`
}

// jwks holds a set of JSON Web Keys.
type jwks struct {
	Keys []jwk `json:"keys"`
}

// jwk represents a single JSON Web Key.
type jwk struct {
	Kty string `json:"kty"`
	Kid string `json:"kid"`
	Use string `json:"use"`
	Alg string `json:"alg"`
	N   string `json:"n"`   // RSA modulus (base64url)
	E   string `json:"e"`   // RSA exponent (base64url)
	Crv string `json:"crv"` // EC curve
	X   string `json:"x"`   // EC x (base64url)
	Y   string `json:"y"`   // EC y (base64url)
}

// jwksCache caches the JWKS document with a TTL.
type jwksCache struct {
	mu        sync.RWMutex
	keys      []jwk
	fetchedAt time.Time
	ttl       time.Duration
}

// OIDCProvider validates OIDC id_tokens and manages the OAuth2 flow.
type OIDCProvider struct {
	cfg        OIDCConfig
	discovery  *oidcDiscovery
	oauth2Cfg  *oauth2.Config
	keyCache   jwksCache
	httpClient *http.Client
}

// NewOIDCProvider creates a new OIDCProvider. It fetches the OIDC discovery
// document from {issuerURL}/.well-known/openid-configuration.
func NewOIDCProvider(cfg OIDCConfig) (*OIDCProvider, error) {
	if cfg.IssuerURL == "" {
		return nil, errors.New("oidc: issuer_url must not be empty")
	}
	if cfg.ClientID == "" {
		return nil, errors.New("oidc: client_id must not be empty")
	}
	if cfg.GroupsClaim == "" {
		cfg.GroupsClaim = "groups"
	}

	p := &OIDCProvider{
		cfg:        cfg,
		httpClient: &http.Client{Timeout: 10 * time.Second},
		keyCache:   jwksCache{ttl: time.Hour},
	}

	disc, err := p.fetchDiscovery(context.Background())
	if err != nil {
		return nil, fmt.Errorf("oidc: fetch discovery document: %w", err)
	}
	p.discovery = disc

	p.oauth2Cfg = &oauth2.Config{
		ClientID:     cfg.ClientID,
		ClientSecret: cfg.ClientSecret,
		RedirectURL:  cfg.RedirectURL,
		Endpoint: oauth2.Endpoint{
			AuthURL:  disc.AuthEndpoint,
			TokenURL: disc.TokenEndpoint,
		},
		Scopes: []string{"openid", "email", "profile"},
	}

	return p, nil
}

// fetchDiscovery retrieves the OIDC discovery document.
func (p *OIDCProvider) fetchDiscovery(ctx context.Context) (*oidcDiscovery, error) {
	url := strings.TrimRight(p.cfg.IssuerURL, "/") + "/.well-known/openid-configuration"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("discovery returned HTTP %d", resp.StatusCode)
	}
	var doc oidcDiscovery
	if err = json.NewDecoder(resp.Body).Decode(&doc); err != nil {
		return nil, fmt.Errorf("decode: %w", err)
	}
	return &doc, nil
}

// fetchJWKS retrieves JWKS from the IdP, using a cached copy if still fresh.
func (p *OIDCProvider) fetchJWKS(ctx context.Context) ([]jwk, error) {
	p.keyCache.mu.RLock()
	if time.Since(p.keyCache.fetchedAt) < p.keyCache.ttl && len(p.keyCache.keys) > 0 {
		keys := p.keyCache.keys
		p.keyCache.mu.RUnlock()
		return keys, nil
	}
	p.keyCache.mu.RUnlock()

	// Fetch fresh JWKS.
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.discovery.JWKSURI, nil)
	if err != nil {
		return nil, fmt.Errorf("build jwks request: %w", err)
	}
	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch jwks: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("jwks returned HTTP %d", resp.StatusCode)
	}
	var ks jwks
	if err = json.NewDecoder(resp.Body).Decode(&ks); err != nil {
		return nil, fmt.Errorf("decode jwks: %w", err)
	}

	p.keyCache.mu.Lock()
	p.keyCache.keys = ks.Keys
	p.keyCache.fetchedAt = time.Now()
	p.keyCache.mu.Unlock()

	return ks.Keys, nil
}

// jwtHeader holds the decoded JOSE header fields we care about.
type jwtHeader struct {
	Alg string `json:"alg"`
	Kid string `json:"kid"`
}

// parseRawJWT base64-decodes all three JWT parts and returns the structured
// header, the raw claims map, the signing input (header.claims), and the
// signature bytes. It does NOT verify the signature or validate any claims.
func parseRawJWT(rawToken string) (jwtHeader, map[string]interface{}, string, []byte, error) {
	parts := strings.Split(rawToken, ".")
	if len(parts) != 3 {
		return jwtHeader{}, nil, "", nil, errors.New("oidc: invalid JWT format")
	}

	headerBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return jwtHeader{}, nil, "", nil, fmt.Errorf("oidc: decode header: %w", err)
	}
	var hdr jwtHeader
	if err = json.Unmarshal(headerBytes, &hdr); err != nil {
		return jwtHeader{}, nil, "", nil, fmt.Errorf("oidc: parse header: %w", err)
	}

	claimsBytes, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return jwtHeader{}, nil, "", nil, fmt.Errorf("oidc: decode claims: %w", err)
	}
	var claims map[string]interface{}
	if err = json.Unmarshal(claimsBytes, &claims); err != nil {
		return jwtHeader{}, nil, "", nil, fmt.Errorf("oidc: parse claims: %w", err)
	}

	sigBytes, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return jwtHeader{}, nil, "", nil, fmt.Errorf("oidc: decode signature: %w", err)
	}
	return hdr, claims, parts[0] + "." + parts[1], sigBytes, nil
}

// VerifyIDToken validates an OIDC id_token JWT. It returns the subject
// (email or sub claim) and the list of groups from the GroupsClaim.
func (p *OIDCProvider) VerifyIDToken(ctx context.Context, rawToken string) (subject string, groups []string, err error) {
	hdr, claims, signingInput, sigBytes, err := parseRawJWT(rawToken)
	if err != nil {
		return "", nil, err
	}

	if err = p.verifyClaims(claims); err != nil {
		return "", nil, err
	}

	keys, err := p.fetchJWKS(ctx)
	if err != nil {
		return "", nil, fmt.Errorf("oidc: fetch jwks: %w", err)
	}

	if err = verifySignatureWithKeys(keys, hdr, []byte(signingInput), sigBytes); err != nil {
		return "", nil, err
	}

	// Extract subject (prefer email over sub).
	sub, _ := claims["sub"].(string)
	if email, ok := claims["email"].(string); ok && email != "" {
		sub = email
	}
	if sub == "" {
		return "", nil, errors.New("oidc: missing sub claim")
	}

	groups = extractStringSlice(claims, p.cfg.GroupsClaim)
	return sub, groups, nil
}

// verifySignatureWithKeys iterates over the JWKS keys and returns nil if any
// key successfully verifies the signature.
func verifySignatureWithKeys(keys []jwk, hdr jwtHeader, signingInput, sigBytes []byte) error {
	for _, k := range keys {
		if hdr.Kid != "" && k.Kid != hdr.Kid {
			continue
		}
		if verifyJWKSignature(k, hdr.Alg, signingInput, sigBytes) == nil {
			return nil
		}
	}
	return errors.New("oidc: signature verification failed")
}

// verifyClaims checks iss, aud, and exp standard claims.
func (p *OIDCProvider) verifyClaims(claims map[string]interface{}) error {
	iss, _ := claims["iss"].(string)
	if iss != p.cfg.IssuerURL && iss != strings.TrimRight(p.cfg.IssuerURL, "/") {
		return fmt.Errorf("oidc: invalid issuer %q", iss)
	}

	switch aud := claims["aud"].(type) {
	case string:
		if aud != p.cfg.ClientID {
			return fmt.Errorf("oidc: invalid audience %q", aud)
		}
	case []interface{}:
		found := false
		for _, a := range aud {
			if s, ok := a.(string); ok && s == p.cfg.ClientID {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("oidc: client_id not in audience")
		}
	default:
		return errors.New("oidc: missing aud claim")
	}

	exp, ok := claims["exp"].(float64)
	if !ok {
		return errors.New("oidc: missing exp claim")
	}
	if time.Now().Unix() > int64(exp) {
		return errors.New("oidc: token has expired")
	}
	return nil
}

// IsAdmin returns true if any group in groups is listed in cfg.AdminGroups.
func (p *OIDCProvider) IsAdmin(groups []string) bool {
	for _, g := range groups {
		for _, ag := range p.cfg.AdminGroups {
			if g == ag {
				return true
			}
		}
	}
	return false
}

// AuthCodeURL returns the OAuth2 authorization URL for the browser redirect flow.
func (p *OIDCProvider) AuthCodeURL(state string) string {
	return p.oauth2Cfg.AuthCodeURL(state, oauth2.AccessTypeOnline)
}

// Exchange exchanges an authorization code for an id_token string.
func (p *OIDCProvider) Exchange(ctx context.Context, code string) (idToken string, err error) {
	token, err := p.oauth2Cfg.Exchange(ctx, code)
	if err != nil {
		return "", fmt.Errorf("oidc: exchange code: %w", err)
	}
	rawIDToken, ok := token.Extra("id_token").(string)
	if !ok || rawIDToken == "" {
		return "", errors.New("oidc: no id_token in token response")
	}
	return rawIDToken, nil
}

// verifyJWKSignature verifies a JWT signature using the given JWK.
func verifyJWKSignature(k jwk, alg string, signingInput, sig []byte) error {
	switch {
	case k.Kty == "RSA" && strings.HasPrefix(alg, "RS"):
		pub, err := rsaPublicKeyFromJWK(k)
		if err != nil {
			return fmt.Errorf("parse rsa key: %w", err)
		}
		return verifyRSASignature(pub, alg, signingInput, sig)
	case k.Kty == "EC" && strings.HasPrefix(alg, "ES"):
		pub, err := ecPublicKeyFromJWK(k)
		if err != nil {
			return fmt.Errorf("parse ec key: %w", err)
		}
		return verifyECSignature(pub, alg, signingInput, sig)
	default:
		return fmt.Errorf("unsupported key type %q / alg %q", k.Kty, alg)
	}
}

// rsaPublicKeyFromJWK constructs an *rsa.PublicKey from a JWK.
func rsaPublicKeyFromJWK(k jwk) (*rsa.PublicKey, error) {
	nBytes, err := base64.RawURLEncoding.DecodeString(k.N)
	if err != nil {
		return nil, fmt.Errorf("decode n: %w", err)
	}
	eBytes, err := base64.RawURLEncoding.DecodeString(k.E)
	if err != nil {
		return nil, fmt.Errorf("decode e: %w", err)
	}
	n := new(big.Int).SetBytes(nBytes)
	var eInt int
	for _, b := range eBytes {
		eInt = eInt<<8 | int(b)
	}
	return &rsa.PublicKey{N: n, E: eInt}, nil
}

// ecPublicKeyFromJWK constructs an *ecdsa.PublicKey from a JWK.
func ecPublicKeyFromJWK(k jwk) (*ecdsa.PublicKey, error) {
	xBytes, err := base64.RawURLEncoding.DecodeString(k.X)
	if err != nil {
		return nil, fmt.Errorf("decode x: %w", err)
	}
	yBytes, err := base64.RawURLEncoding.DecodeString(k.Y)
	if err != nil {
		return nil, fmt.Errorf("decode y: %w", err)
	}
	curve, err := curveForCrv(k.Crv)
	if err != nil {
		return nil, err
	}
	return &ecdsa.PublicKey{
		Curve: curve,
		X:     new(big.Int).SetBytes(xBytes),
		Y:     new(big.Int).SetBytes(yBytes),
	}, nil
}

// extractStringSlice extracts a string slice from a claim that may be
// a []interface{} or a single string.
func extractStringSlice(claims map[string]interface{}, key string) []string {
	v, ok := claims[key]
	if !ok {
		return nil
	}
	switch val := v.(type) {
	case string:
		if val == "" {
			return nil
		}
		return []string{val}
	case []interface{}:
		out := make([]string, 0, len(val))
		for _, item := range val {
			if s, ok := item.(string); ok && s != "" {
				out = append(out, s)
			}
		}
		return out
	default:
		return nil
	}
}
