package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/gluk-w/claworc/control-plane/internal/auth"
	"github.com/gluk-w/claworc/control-plane/internal/database"
	"github.com/gluk-w/claworc/control-plane/internal/middleware"
)

// ListAPITokens returns the caller's tokens without the token secret.
// GET /api/v1/auth/tokens → 200 [{id,name,prefix,last_used_at?,expires_at?,created_at}]
func ListAPITokens(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r)
	if user == nil {
		writeError(w, http.StatusUnauthorized, "Authentication required")
		return
	}

	tokens, err := database.ListAPITokensByUser(user.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to list tokens")
		return
	}

	type tokenResponse struct {
		ID         uint    `json:"id"`
		Name       string  `json:"name"`
		Prefix     string  `json:"prefix"`
		LastUsedAt *string `json:"last_used_at,omitempty"`
		ExpiresAt  *string `json:"expires_at,omitempty"`
		CreatedAt  string  `json:"created_at"`
	}

	result := make([]tokenResponse, 0, len(tokens))
	for _, t := range tokens {
		resp := tokenResponse{
			ID:        t.ID,
			Name:      t.Name,
			Prefix:    t.Prefix,
			CreatedAt: formatTimestamp(t.CreatedAt),
		}
		if t.LastUsedAt != nil {
			s := formatTimestamp(*t.LastUsedAt)
			resp.LastUsedAt = &s
		}
		if t.ExpiresAt != nil {
			s := formatTimestamp(*t.ExpiresAt)
			resp.ExpiresAt = &s
		}
		result = append(result, resp)
	}

	writeJSON(w, http.StatusOK, result)
}

// CreateAPIToken mints a new token and returns the plaintext exactly once.
// POST /api/v1/auth/tokens {"name":string,"expires_in_days"?:number} → 201
func CreateAPIToken(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r)
	if user == nil {
		writeError(w, http.StatusUnauthorized, "Authentication required")
		return
	}

	var body struct {
		Name          string `json:"name"`
		ExpiresInDays int    `json:"expires_in_days"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if body.Name == "" {
		writeError(w, http.StatusBadRequest, "Name is required")
		return
	}

	plain, hash, prefix, err := auth.GenerateAPIToken()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to generate token")
		return
	}

	tok := &database.APIToken{
		UserID:    user.ID,
		Name:      body.Name,
		TokenHash: hash,
		Prefix:    prefix,
	}
	if body.ExpiresInDays > 0 {
		t := time.Now().AddDate(0, 0, body.ExpiresInDays)
		tok.ExpiresAt = &t
	}

	if err := database.CreateAPIToken(tok); err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to create token")
		return
	}

	type createResponse struct {
		ID        uint    `json:"id"`
		Name      string  `json:"name"`
		Token     string  `json:"token"`
		Prefix    string  `json:"prefix"`
		ExpiresAt *string `json:"expires_at,omitempty"`
		CreatedAt string  `json:"created_at"`
	}

	resp := createResponse{
		ID:        tok.ID,
		Name:      tok.Name,
		Token:     plain,
		Prefix:    tok.Prefix,
		CreatedAt: formatTimestamp(tok.CreatedAt),
	}
	if tok.ExpiresAt != nil {
		s := formatTimestamp(*tok.ExpiresAt)
		resp.ExpiresAt = &s
	}

	writeJSON(w, http.StatusCreated, resp)
}

// DeleteAPIToken revokes a token by ID, scoped to the caller.
// DELETE /api/v1/auth/tokens/{id} → 204
func DeleteAPIToken(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r)
	if user == nil {
		writeError(w, http.StatusUnauthorized, "Authentication required")
		return
	}

	idStr := chi.URLParam(r, "id")
	id, err := strconv.Atoi(idStr)
	if err != nil || id <= 0 {
		writeError(w, http.StatusBadRequest, "Invalid token ID")
		return
	}

	if err := database.DeleteAPIToken(uint(id), user.ID); err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to delete token")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
