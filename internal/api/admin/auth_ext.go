package admin

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"net/http"

	"github.com/madhavkobal/sangraha/internal/auth"
)

// authExtHandler handles external authentication flows (OIDC, LDAP).
type authExtHandler struct {
	oidc     *auth.OIDCProvider
	ldap     *auth.LDAPProvider
	keyStore *auth.KeyStore
}

// oidcURLResponse is the body for GET /admin/v1/auth/oidc/url.
type oidcURLResponse struct {
	URL   string `json:"url"`
	State string `json:"state"`
}

// oidcCallbackRequest is the body for POST /admin/v1/auth/oidc/callback.
type oidcCallbackRequest struct {
	Code  string `json:"code"`
	State string `json:"state"`
}

// oidcCallbackResponse is returned after a successful OIDC exchange.
type oidcCallbackResponse struct {
	Subject string   `json:"subject"`
	IsAdmin bool     `json:"is_admin"`
	Groups  []string `json:"groups"`
}

// ldapAuthRequest is the body for POST /admin/v1/auth/ldap.
type ldapAuthRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// ldapAuthResponse is returned after a successful LDAP authentication.
type ldapAuthResponse struct {
	Username string   `json:"username"`
	IsAdmin  bool     `json:"is_admin"`
	Groups   []string `json:"groups"`
}

// handleOIDCURL returns the authorization URL the browser should redirect to.
func (h *authExtHandler) handleOIDCURL(w http.ResponseWriter, r *http.Request) {
	if h.oidc == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "OIDC not configured"})
		return
	}
	// Generate a random state value to protect against CSRF.
	state, err := oidcRandomState()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to generate state"})
		return
	}
	url := h.oidc.AuthCodeURL(state)
	writeJSON(w, http.StatusOK, oidcURLResponse{URL: url, State: state})
}

// handleOIDCCallback exchanges an authorization code for an id_token and
// returns the subject and group information.
func (h *authExtHandler) handleOIDCCallback(w http.ResponseWriter, r *http.Request) {
	if h.oidc == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "OIDC not configured"})
		return
	}
	var req oidcCallbackRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}
	if req.Code == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "code is required"})
		return
	}

	idToken, err := h.oidc.Exchange(r.Context(), req.Code)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "code exchange failed: " + err.Error()})
		return
	}

	subject, groups, err := h.oidc.VerifyIDToken(r.Context(), idToken)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "token verification failed: " + err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, oidcCallbackResponse{
		Subject: subject,
		IsAdmin: h.oidc.IsAdmin(groups),
		Groups:  groups,
	})
}

// handleLDAPAuth authenticates a user via LDAP and returns group information.
func (h *authExtHandler) handleLDAPAuth(w http.ResponseWriter, r *http.Request) {
	if h.ldap == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "LDAP not configured"})
		return
	}
	var req ldapAuthRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}
	if req.Username == "" || req.Password == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "username and password are required"})
		return
	}

	groups, err := h.ldap.Authenticate(r.Context(), req.Username, req.Password)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "authentication failed"})
		return
	}

	writeJSON(w, http.StatusOK, ldapAuthResponse{
		Username: req.Username,
		IsAdmin:  h.ldap.IsAdmin(groups),
		Groups:   groups,
	})
}

// oidcRandomState generates a cryptographically random state string for OAuth2 CSRF protection.
func oidcRandomState() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
