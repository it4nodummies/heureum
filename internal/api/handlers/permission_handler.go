package handlers

import (
	"net/http"
	"strconv"

	"github.com/it4nodummies/heureum/internal/api/middleware"
	v3 "github.com/it4nodummies/heureum/internal/api/v3"
	"github.com/it4nodummies/heureum/internal/domain/permission"
	"github.com/it4nodummies/heureum/internal/domain/project"
	"github.com/it4nodummies/heureum/internal/domain/user"
	"gorm.io/gorm"
)

type PermissionHandler struct {
	db         *gorm.DB
	projectSvc *project.Service
}

func NewPermissionHandler(db *gorm.DB, projectSvc *project.Service) *PermissionHandler {
	return &PermissionHandler{db: db, projectSvc: projectSvc}
}

// userPermission è lo shape UserPermission del contratto.
type userPermission struct {
	ID             string `json:"id"`
	Key            string `json:"key"`
	Name           string `json:"name"`
	Description    string `json:"description"`
	Type           string `json:"type"`
	HavePermission bool   `json:"havePermission,omitempty"`
}

// Permissions: GET /rest/api/3/permissions → tutte le chiavi (senza havePermission).
func (h *PermissionHandler) Permissions(w http.ResponseWriter, r *http.Request) {
	perms := map[string]userPermission{}
	for i, d := range permission.All() {
		perms[d.Key] = userPermission{ID: strconv.Itoa(i + 1), Key: d.Key, Name: d.Name, Description: d.Description, Type: d.Type}
	}
	v3.WriteJSON(w, http.StatusOK, map[string]any{"permissions": perms})
}

// MyPermissions: GET /rest/api/3/mypermissions?projectKey=... → havePermission
// derivato da is_admin globale + ruolo nel progetto (se projectKey presente).
func (h *PermissionHandler) MyPermissions(w http.ResponseWriter, r *http.Request) {
	uid := middleware.UserIDFromContext(r.Context())
	var u user.User
	isAdmin := false
	if h.db.First(&u, "id = ?", uid).Error == nil {
		isAdmin = u.IsAdmin
	}
	role := ""
	if key := r.URL.Query().Get("projectKey"); key != "" {
		if p, err := h.projectSvc.GetByKey(key); err == nil {
			var m struct{ Role string }
			h.db.Table("project_members").Select("role").Where("project_id = ? AND user_id = ?", p.ID, uid).Scan(&m)
			role = m.Role
		}
	}
	have := permission.ForRole(role, isAdmin)
	perms := map[string]userPermission{}
	for i, d := range permission.All() {
		perms[d.Key] = userPermission{ID: strconv.Itoa(i + 1), Key: d.Key, Name: d.Name, Description: d.Description, Type: d.Type, HavePermission: have[d.Key]}
	}
	v3.WriteJSON(w, http.StatusOK, map[string]any{"permissions": perms})
}
