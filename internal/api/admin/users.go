package admin

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/madhavkobal/sangraha/internal/auth"
)

// userHandler provides user management endpoints.
type userHandler struct {
	keyStore *auth.KeyStore
}

// createUserRequest is the JSON body for POST /admin/v1/users.
type createUserRequest struct {
	Owner string `json:"owner"`
}

// createUserResponse is the JSON response for a newly created user.
type createUserResponse struct {
	AccessKey string `json:"access_key"` //nolint:gosec // G101: field name matches pattern but is not a hardcoded credential
	SecretKey string `json:"secret_key"` //nolint:gosec // G101: field name matches pattern but is not a hardcoded credential
	Owner     string `json:"owner"`
}

func (h *userHandler) create(w http.ResponseWriter, r *http.Request) {
	var req createUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}
	if req.Owner == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "owner is required"})
		return
	}
	ak, sk, err := h.keyStore.CreateKey(r.Context(), req.Owner, false)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusCreated, createUserResponse{
		AccessKey: ak,
		SecretKey: sk,
		Owner:     req.Owner,
	})
}

func (h *userHandler) list(w http.ResponseWriter, r *http.Request) {
	keys, err := h.keyStore.ListKeys(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	type userInfo struct {
		AccessKey string `json:"access_key"` //nolint:gosec // G101: field name matches pattern but is not a hardcoded credential
		Owner     string `json:"owner"`
		IsRoot    bool   `json:"is_root"`
	}
	out := make([]userInfo, len(keys))
	for i, k := range keys {
		out[i] = userInfo{AccessKey: k.AccessKey, Owner: k.Owner, IsRoot: k.IsRoot}
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *userHandler) delete(w http.ResponseWriter, r *http.Request) {
	ak := chi.URLParam(r, "accessKey")
	if err := h.keyStore.DeleteKey(r.Context(), ak); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *userHandler) rotateKey(w http.ResponseWriter, r *http.Request) {
	ak := chi.URLParam(r, "accessKey")
	existing, err := h.keyStore.Lookup(r.Context(), ak)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "access key not found"})
		return
	}
	if err = h.keyStore.DeleteKey(r.Context(), ak); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	newAK, newSK, err := h.keyStore.CreateKey(r.Context(), existing.Owner, existing.IsRoot)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, createUserResponse{
		AccessKey: newAK,
		SecretKey: newSK,
		Owner:     existing.Owner,
	})
}
