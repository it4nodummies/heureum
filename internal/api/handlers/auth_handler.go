package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/it4nodummies/heureum/internal/api/middleware"
	v3 "github.com/it4nodummies/heureum/internal/api/v3"
	"github.com/it4nodummies/heureum/internal/domain/auth"
)

type AuthHandler struct {
	svc        *auth.Service
	signupOpen bool
}

// NewAuthHandler builds the auth handler. signupOpen controls whether
// Register accepts new users; when false, Register returns 403 — except for
// the very first user of an empty instance, who is always accepted and
// becomes admin (see auth.Service.RegisterWithPolicy).
func NewAuthHandler(svc *auth.Service, signupOpen bool) *AuthHandler {
	return &AuthHandler{svc: svc, signupOpen: signupOpen}
}

func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email    string `json:"email"`
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}
	if req.Email == "" || req.Username == "" || req.Password == "" {
		http.Error(w, `{"error":"email, username, and password are required"}`, http.StatusBadRequest)
		return
	}
	u, err := h.svc.RegisterWithPolicy(req.Email, req.Username, req.Username, req.Password, h.signupOpen)
	if errors.Is(err, auth.ErrSignupClosed) {
		v3.WriteError(w, http.StatusForbidden, []string{"signup is disabled on this instance"}, nil)
		return
	}
	if err != nil {
		http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusConflict)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(u)
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}
	token, err := h.svc.Login(req.Email, req.Password)
	if err != nil {
		http.Error(w, `{"error":"invalid credentials"}`, http.StatusUnauthorized)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"token": token})
}

// apiTokenView is the wire shape of an API token on the list/create routes.
// It is an explicit DTO (not the domain struct) so that the hash can never be
// serialized by accident; the plaintext is only ever set by CreateAPIToken.
type apiTokenView struct {
	ID         string     `json:"id"`
	Label      string     `json:"label"`
	Token      string     `json:"token,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
	LastUsedAt *time.Time `json:"last_used_at,omitempty"`
	ExpiresAt  *time.Time `json:"expires_at,omitempty"`
}

func toAPITokenView(t auth.APIToken) apiTokenView {
	return apiTokenView{ID: t.ID, Label: t.Label, CreatedAt: t.CreatedAt, LastUsedAt: t.LastUsedAt, ExpiresAt: t.ExpiresAt}
}

// CreateAPIToken creates a personal API token (in real Jira Cloud, these are
// managed on id.atlassian.com; here they are self-service). An optional
// expiresInDays (> 0) sets expires_at = now + N days; omitted means the token
// never expires.
func (h *AuthHandler) CreateAPIToken(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	var req struct {
		Label         string `json:"label"`
		ExpiresInDays *int   `json:"expiresInDays"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		v3.WriteError(w, http.StatusBadRequest, []string{"Invalid request body."}, nil)
		return
	}
	if len(req.Label) > 255 {
		v3.WriteError(w, http.StatusBadRequest, nil,
			map[string]string{"label": "Label must be at most 255 characters."})
		return
	}
	var expiresAt *time.Time
	if req.ExpiresInDays != nil {
		if *req.ExpiresInDays <= 0 {
			v3.WriteError(w, http.StatusBadRequest, nil,
				map[string]string{"expiresInDays": "expiresInDays must be a positive number of days."})
			return
		}
		// Clock() is UTC: the column is a tz-less TIMESTAMP, see auth.Service.Clock.
		t := h.svc.Clock().Add(time.Duration(*req.ExpiresInDays) * 24 * time.Hour)
		expiresAt = &t
	}
	tok, plaintext, err := h.svc.CreateAPIToken(userID, req.Label, expiresAt)
	if err != nil {
		v3.WriteError(w, http.StatusInternalServerError, []string{"Failed to create API token."}, nil)
		return
	}
	view := toAPITokenView(*tok)
	view.Token = plaintext
	v3.WriteJSON(w, http.StatusCreated, view)
}

// ListAPITokens returns the caller's own API tokens: metadata only, never the
// hash or the plaintext.
func (h *AuthHandler) ListAPITokens(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	tokens, err := h.svc.ListAPITokens(userID)
	if err != nil {
		v3.WriteError(w, http.StatusInternalServerError, []string{"Failed to list API tokens."}, nil)
		return
	}
	views := make([]apiTokenView, 0, len(tokens))
	for _, t := range tokens {
		views = append(views, toAPITokenView(t))
	}
	v3.WriteJSON(w, http.StatusOK, views)
}

// RevokeAPIToken deletes one of the caller's tokens. An id that is unknown or
// belongs to another user is a 404 either way (no cross-account probing).
func (h *AuthHandler) RevokeAPIToken(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	id := r.PathValue("id")
	err := h.svc.RevokeAPIToken(userID, id)
	if errors.Is(err, auth.ErrTokenNotFound) {
		v3.WriteError(w, http.StatusNotFound, []string{"API token not found."}, nil)
		return
	}
	if err != nil {
		v3.WriteError(w, http.StatusInternalServerError, []string{"Failed to revoke API token."}, nil)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
