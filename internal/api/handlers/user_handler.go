package handlers

import (
	"encoding/json"
	"net/http"

	"gorm.io/gorm"

	"github.com/open-jira/open-jira/internal/api/middleware"
	"github.com/open-jira/open-jira/internal/domain/user"
)

type UserHandler struct {
	DB *gorm.DB
}

func NewUserHandler(db *gorm.DB) *UserHandler {
	return &UserHandler{DB: db}
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
