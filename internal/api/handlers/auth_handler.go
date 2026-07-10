package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/open-jira/open-jira/internal/api/middleware"
	v3 "github.com/open-jira/open-jira/internal/api/v3"
	"github.com/open-jira/open-jira/internal/domain/auth"
)

type AuthHandler struct {
	svc *auth.Service
}

func NewAuthHandler(svc *auth.Service) *AuthHandler {
	return &AuthHandler{svc: svc}
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
	u, err := h.svc.Register(req.Email, req.Username, req.Username, req.Password)
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

// CreateAPIToken creates a personal API token (in real Jira Cloud, these are
// managed on id.atlassian.com; here they are self-service).
func (h *AuthHandler) CreateAPIToken(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	var req struct {
		Label string `json:"label"`
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
	tok, plaintext, err := h.svc.CreateAPIToken(userID, req.Label)
	if err != nil {
		v3.WriteError(w, http.StatusInternalServerError, []string{"Failed to create API token."}, nil)
		return
	}
	v3.WriteJSON(w, http.StatusCreated, struct {
		ID        string    `json:"id"`
		Label     string    `json:"label"`
		Token     string    `json:"token"`
		CreatedAt time.Time `json:"created_at"`
	}{ID: tok.ID, Label: tok.Label, Token: plaintext, CreatedAt: tok.CreatedAt})
}
