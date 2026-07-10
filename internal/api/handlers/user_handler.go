package handlers

import (
	"encoding/json"
	"net/http"

	"gorm.io/gorm"

	"github.com/open-jira/open-jira/internal/api/middleware"
	v3 "github.com/open-jira/open-jira/internal/api/v3"
	"github.com/open-jira/open-jira/internal/domain/user"
)

type UserHandler struct {
	DB      *gorm.DB
	BaseURL string
}

// NewUserHandler costruisce lo handler. baseURL è opzionale (variadico) per
// non rompere le chiamate esistenti (es. NewUserHandler(db) nei test);
// serve solo a GetMyself per costruire i link "self" nel formato Jira v3.
func NewUserHandler(db *gorm.DB, baseURL ...string) *UserHandler {
	bu := ""
	if len(baseURL) > 0 {
		bu = baseURL[0]
	}
	return &UserHandler{DB: db, BaseURL: bu}
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
