package handlers

import (
	"encoding/json"
	"net/http"

	"gorm.io/gorm"

	"github.com/it4nodummies/heureum/internal/api/authz"
	"github.com/it4nodummies/heureum/internal/api/middleware"
	v3 "github.com/it4nodummies/heureum/internal/api/v3"
	"github.com/it4nodummies/heureum/internal/domain/permission"
	"github.com/it4nodummies/heureum/internal/domain/project"
	"github.com/it4nodummies/heureum/internal/domain/user"
)

type UserHandler struct {
	DB      *gorm.DB
	BaseURL string
	chk     *authz.Checker
}

// NewUserHandler costruisce lo handler. baseURL serve a GetMyself per
// costruire i link "self" e gli avatar nel formato Jira v3 (stessa
// convenzione posizionale di NewOAuthHandler). chk serve a sanitizzare la
// directory utenti (email solo a sé/admin) e a verificare l'appartenenza al
// progetto in AssignableSearch (Task 8, Round 12).
func NewUserHandler(db *gorm.DB, baseURL string, chk *authz.Checker) *UserHandler {
	return &UserHandler{DB: db, BaseURL: baseURL, chk: chk}
}

// directoryUser è la shape sanitizzata della directory utenti (SearchUsers,
// GetUser): mai serializzare user.User raw, che include email, password
// hash e flag is_admin. Email è inclusa solo se il chiamante è autorizzato
// a vederla (admin globale, o accountId == se stesso per GetUser).
type directoryUser struct {
	ID          string `json:"id"`
	Username    string `json:"username"`
	DisplayName string `json:"display_name"`
	AvatarURL   string `json:"avatar_url"`
	IsActive    bool   `json:"is_active"`
	TimeZone    string `json:"time_zone"`
	Locale      string `json:"locale"`
	Email       string `json:"email,omitempty"`
}

func sanitizeDirectoryUser(u user.User, includeEmail bool) directoryUser {
	out := directoryUser{
		ID:          u.ID,
		Username:    u.Username,
		DisplayName: u.DisplayName,
		AvatarURL:   u.AvatarURL,
		IsActive:    u.IsActive,
		TimeZone:    u.TimeZone,
		Locale:      u.Locale,
	}
	if includeEmail {
		out.Email = u.Email
	}
	return out
}

func (h *UserHandler) isGlobalAdmin(uid string) bool {
	return h.chk != nil && h.chk.IsGlobalAdmin(uid)
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
	uid := middleware.UserIDFromContext(r.Context())
	query := r.URL.Query().Get("query")
	var users []user.User
	db := h.DB.Model(&user.User{})
	if query != "" {
		like := "%" + query + "%"
		db = db.Where("username LIKE ? OR display_name LIKE ? OR email LIKE ?", like, like, like)
	}
	db.Limit(50).Find(&users)

	includeEmail := h.isGlobalAdmin(uid)
	out := make([]directoryUser, 0, len(users))
	for _, u := range users {
		out = append(out, sanitizeDirectoryUser(u, includeEmail))
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(out)
}

// SearchV3: GET /rest/api/3/user/search?query=... → []v3.User (shape contratto).
// Email è azzerata prima del mapping se il chiamante non è admin globale
// (EmailAddress ha omitempty nel DTO v3.User → il campo sparisce dal JSON).
func (h *UserHandler) SearchV3(w http.ResponseWriter, r *http.Request) {
	uid := middleware.UserIDFromContext(r.Context())
	svc := user.NewService(h.DB)
	users, err := svc.Search(r.URL.Query().Get("query"), 50)
	if err != nil {
		v3.WriteError(w, http.StatusInternalServerError, []string{"search failed"}, nil)
		return
	}
	includeEmail := h.isGlobalAdmin(uid)
	out := make([]v3.User, 0, len(users))
	for _, u := range users {
		if !includeEmail {
			u.Email = ""
		}
		out = append(out, v3.JiraUser(u, h.BaseURL))
	}
	v3.WriteJSON(w, http.StatusOK, out)
}

// AssignableSearch: GET /rest/api/3/user/assignable/search?project=KEY&query=...
// Il chiamante deve poter vedere il progetto (BROWSE_PROJECTS): senza,
// risponde 404 (stesso stile di EnforceNotFound, per non rivelare tramite
// un 403 l'esistenza del progetto).
func (h *UserHandler) AssignableSearch(w http.ResponseWriter, r *http.Request) {
	uid := middleware.UserIDFromContext(r.Context())
	key := r.URL.Query().Get("project")
	var p project.Project
	if h.DB.First(&p, "key = ?", key).Error != nil {
		v3.WriteError(w, http.StatusNotFound, []string{"project not found"}, nil)
		return
	}
	if h.chk != nil {
		if err := h.chk.RequireProject(uid, p.ID, permission.BrowseProjects); err != nil {
			v3.WriteError(w, http.StatusNotFound, []string{"the resource does not exist or you do not have permission to view it"}, nil)
			return
		}
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
	uid := middleware.UserIDFromContext(r.Context())
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
	includeEmail := h.isGlobalAdmin(uid) || accountID == uid
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(sanitizeDirectoryUser(u, includeEmail))
}
