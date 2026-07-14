package handlers

import (
	"encoding/json"
	"net/http"

	"gorm.io/gorm"

	"github.com/it4nodummies/heureum/internal/api/middleware"
	v3 "github.com/it4nodummies/heureum/internal/api/v3"
	"github.com/it4nodummies/heureum/internal/domain/project"
	"github.com/it4nodummies/heureum/internal/domain/user"
)

type UserHandler struct {
	DB      *gorm.DB
	BaseURL string
}

// NewUserHandler costruisce lo handler. baseURL serve a GetMyself per
// costruire i link "self" e gli avatar nel formato Jira v3 (stessa
// convenzione posizionale di NewOAuthHandler).
func NewUserHandler(db *gorm.DB, baseURL string) *UserHandler {
	return &UserHandler{DB: db, BaseURL: baseURL}
}

func (h *UserHandler) GetMe(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	if userID == "" {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}
	var u user.User
	if err := h.DB.First(&u, "id = ?", userID).Error; err != nil {
		http.Error(w, `{"error":"user not found"}`, http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(u)
}

// GetMyself risponde a GET /rest/api/3/myself nel formato Jira v3 (schema User).
func (h *UserHandler) GetMyself(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	var u user.User
	if err := h.DB.First(&u, "id = ?", userID).Error; err != nil {
		// 401 (non 404 come GetMe): un token valido il cui principal non
		// esiste più è un fallimento di autenticazione per /myself.
		v3.WriteError(w, http.StatusUnauthorized, []string{"The user does not exist."}, nil)
		return
	}
	v3.WriteJSON(w, http.StatusOK, v3.JiraUser(u, h.BaseURL))
}

func (h *UserHandler) SearchUsers(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("query")
	var users []user.User
	db := h.DB.Model(&user.User{})
	if query != "" {
		like := "%" + query + "%"
		db = db.Where("username LIKE ? OR display_name LIKE ? OR email LIKE ?", like, like, like)
	}
	db.Limit(50).Find(&users)
	if users == nil {
		users = []user.User{}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(users)
}

// SearchV3: GET /rest/api/3/user/search?query=... → []v3.User (shape contratto).
func (h *UserHandler) SearchV3(w http.ResponseWriter, r *http.Request) {
	svc := user.NewService(h.DB)
	users, err := svc.Search(r.URL.Query().Get("query"), 50)
	if err != nil {
		v3.WriteError(w, http.StatusInternalServerError, []string{"search failed"}, nil)
		return
	}
	out := make([]v3.User, 0, len(users))
	for _, u := range users {
		out = append(out, v3.JiraUser(u, h.BaseURL))
	}
	v3.WriteJSON(w, http.StatusOK, out)
}

// AssignableSearch: GET /rest/api/3/user/assignable/search?project=KEY&query=...
func (h *UserHandler) AssignableSearch(w http.ResponseWriter, r *http.Request) {
	key := r.URL.Query().Get("project")
	var p project.Project
	if h.DB.First(&p, "key = ?", key).Error != nil {
		v3.WriteError(w, http.StatusNotFound, []string{"project not found"}, nil)
		return
	}
	svc := user.NewService(h.DB)
	users, err := svc.AssignableForProject(p.ID, r.URL.Query().Get("query"), 50)
	if err != nil {
		v3.WriteError(w, http.StatusInternalServerError, []string{"search failed"}, nil)
		return
	}
	out := make([]v3.User, 0, len(users))
	for _, u := range users {
		out = append(out, v3.JiraUser(u, h.BaseURL))
	}
	v3.WriteJSON(w, http.StatusOK, out)
}

// UpdateMyself: PUT /rest/api/3/myself {displayName?, timeZone?, locale?, avatarUrl?}
// (estensione: Jira non ha PUT /myself, ma serve per l'editing profilo).
func (h *UserHandler) UpdateMyself(w http.ResponseWriter, r *http.Request) {
	uid := middleware.UserIDFromContext(r.Context())
	var req struct {
		DisplayName *string `json:"displayName"`
		TimeZone    *string `json:"timeZone"`
		Locale      *string `json:"locale"`
		AvatarURL   *string `json:"avatarUrl"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		v3.WriteError(w, http.StatusBadRequest, []string{"invalid request body"}, nil)
		return
	}
	svc := user.NewService(h.DB)
	u, err := svc.UpdateProfile(uid, req.DisplayName, req.TimeZone, req.Locale, req.AvatarURL)
	if err != nil {
		v3.WriteError(w, http.StatusInternalServerError, []string{"failed to update profile"}, nil)
		return
	}
	v3.WriteJSON(w, http.StatusOK, v3.JiraUser(*u, h.BaseURL))
}

func (h *UserHandler) GetUser(w http.ResponseWriter, r *http.Request) {
	accountID := r.URL.Query().Get("accountId")
	if accountID == "" {
		http.Error(w, `{"error":"accountId is required"}`, http.StatusBadRequest)
		return
	}
	var u user.User
	if err := h.DB.First(&u, "id = ?", accountID).Error; err != nil {
		http.Error(w, `{"error":"user not found"}`, http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(u)
}
