package handlers

import (
	"encoding/json"
	"net/http"

	v3 "github.com/it4nodummies/heureum/internal/api/v3"
	"github.com/it4nodummies/heureum/internal/domain/group"
	"github.com/it4nodummies/heureum/internal/domain/project"
)

// ProjectTeamHandler serves the Heureum extension endpoints that associate a
// team (group) to a project with a role:
//
//	GET    /rest/api/3/project/{key}/teams
//	POST   /rest/api/3/project/{key}/teams            {groupId, role}
//	PUT    /rest/api/3/project/{key}/teams/{groupId}  {role}
//	DELETE /rest/api/3/project/{key}/teams/{groupId}
//
// It mirrors the project member endpoints (ListMembers/AddMember/RemoveMember
// on ProjectHandler): the same GetByKey project resolution, and it is keyed on
// {key} and {groupId} only — never on an internal UUID.
type ProjectTeamHandler struct {
	svc *project.Service
}

func NewProjectTeamHandler(svc *project.Service) *ProjectTeamHandler {
	return &ProjectTeamHandler{svc: svc}
}

// validTeamRole reports whether r is one of the accepted project roles.
func validTeamRole(r project.MemberRole) bool {
	switch r {
	case project.RoleAdmin, project.RoleMember, project.RoleViewer:
		return true
	default:
		return false
	}
}

// groupExists reports whether a group row with id exists. It queries the shared
// project DB handle (like GroupHandler.Members) so the handler needs no extra
// service dependency.
func (h *ProjectTeamHandler) groupExists(id string) bool {
	if id == "" {
		return false
	}
	var count int64
	h.svc.DB().Model(&group.Group{}).Where("id = ?", id).Count(&count)
	return count > 0
}

// List: GET /project/{key}/teams → 200 [{groupId,name,role}].
func (h *ProjectTeamHandler) List(w http.ResponseWriter, r *http.Request) {
	p, err := h.svc.GetByKey(r.PathValue("key"))
	if err != nil {
		v3.WriteError(w, http.StatusNotFound, []string{"project not found"}, nil)
		return
	}
	teams, err := h.svc.ListTeams(p.ID)
	if err != nil {
		v3.WriteError(w, http.StatusInternalServerError, []string{"failed to list teams"}, nil)
		return
	}
	if teams == nil {
		teams = []project.ProjectTeamInfo{}
	}
	v3.WriteJSON(w, http.StatusOK, teams)
}

// upsertTeam validates the role and that the group exists, then upserts the
// association. AddTeam is an idempotent upsert, so once role and group are
// validated the only remaining failure is infrastructure (500) — never a real
// conflict. Shared by Add (groupId from body) and UpdateRole (groupId from path).
func (h *ProjectTeamHandler) upsertTeam(w http.ResponseWriter, projectID, groupID string, role project.MemberRole) {
	if !validTeamRole(role) {
		v3.WriteError(w, http.StatusBadRequest, []string{"role must be one of admin, member, viewer"}, nil)
		return
	}
	if !h.groupExists(groupID) {
		v3.WriteError(w, http.StatusBadRequest, []string{"group not found"}, nil)
		return
	}
	if err := h.svc.AddTeam(projectID, groupID, role); err != nil {
		v3.WriteError(w, http.StatusInternalServerError, []string{"failed to associate team"}, nil)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// Add: POST /project/{key}/teams {groupId, role} → 204. Validates the role and
// that the group exists before upserting the association.
func (h *ProjectTeamHandler) Add(w http.ResponseWriter, r *http.Request) {
	p, err := h.svc.GetByKey(r.PathValue("key"))
	if err != nil {
		v3.WriteError(w, http.StatusNotFound, []string{"project not found"}, nil)
		return
	}
	var req struct {
		GroupID string             `json:"groupId"`
		Role    project.MemberRole `json:"role"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		v3.WriteError(w, http.StatusBadRequest, []string{"invalid request body"}, nil)
		return
	}
	h.upsertTeam(w, p.ID, req.GroupID, req.Role)
}

// UpdateRole: PUT /project/{key}/teams/{groupId} {role} → 204 (AddTeam upsert).
func (h *ProjectTeamHandler) UpdateRole(w http.ResponseWriter, r *http.Request) {
	p, err := h.svc.GetByKey(r.PathValue("key"))
	if err != nil {
		v3.WriteError(w, http.StatusNotFound, []string{"project not found"}, nil)
		return
	}
	var req struct {
		Role project.MemberRole `json:"role"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		v3.WriteError(w, http.StatusBadRequest, []string{"invalid request body"}, nil)
		return
	}
	h.upsertTeam(w, p.ID, r.PathValue("groupId"), req.Role)
}

// Remove: DELETE /project/{key}/teams/{groupId} → 204.
func (h *ProjectTeamHandler) Remove(w http.ResponseWriter, r *http.Request) {
	p, err := h.svc.GetByKey(r.PathValue("key"))
	if err != nil {
		v3.WriteError(w, http.StatusNotFound, []string{"project not found"}, nil)
		return
	}
	if err := h.svc.RemoveTeam(p.ID, r.PathValue("groupId")); err != nil {
		v3.WriteError(w, http.StatusInternalServerError, []string{"failed to remove team"}, nil)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
