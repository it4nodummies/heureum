package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"

	v3 "github.com/it4nodummies/heureum/internal/api/v3"
	"github.com/it4nodummies/heureum/internal/domain/group"
	"github.com/it4nodummies/heureum/internal/domain/user"
	"gorm.io/gorm"
)

type GroupHandler struct {
	svc     *group.Service
	db      *gorm.DB
	baseURL string
}

func NewGroupHandler(svc *group.Service, db *gorm.DB, baseURL string) *GroupHandler {
	return &GroupHandler{svc: svc, db: db, baseURL: baseURL}
}

// Get: GET /rest/api/3/group?groupname=... → GroupRef.
func (h *GroupHandler) Get(w http.ResponseWriter, r *http.Request) {
	name := r.URL.Query().Get("groupname")
	g, err := h.svc.FindByName(name)
	if err != nil {
		v3.WriteError(w, http.StatusNotFound, []string{"group not found"}, nil)
		return
	}
	v3.WriteJSON(w, http.StatusOK, v3.JiraGroup(g.ID, g.Name, h.baseURL))
}

// Create: POST /rest/api/3/group {name} → 201 GroupRef.
func (h *GroupHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Name == "" {
		v3.WriteError(w, http.StatusBadRequest, []string{"name is required"}, nil)
		return
	}
	g, err := h.svc.Create(req.Name)
	if err != nil {
		v3.WriteError(w, http.StatusBadRequest, []string{"failed to create group (duplicate?)"}, nil)
		return
	}
	v3.WriteJSON(w, http.StatusCreated, v3.JiraGroup(g.ID, g.Name, h.baseURL))
}

// Delete: DELETE /rest/api/3/group?groupname=... → 200.
func (h *GroupHandler) Delete(w http.ResponseWriter, r *http.Request) {
	g, err := h.svc.FindByName(r.URL.Query().Get("groupname"))
	if err != nil {
		v3.WriteError(w, http.StatusNotFound, []string{"group not found"}, nil)
		return
	}
	if err := h.svc.Delete(g.ID); err != nil {
		v3.WriteError(w, http.StatusInternalServerError, []string{"failed to delete group"}, nil)
		return
	}
	w.WriteHeader(http.StatusOK)
}

// Members: GET /rest/api/3/group/member?groupname=... → PageBeanUserDetails.
func (h *GroupHandler) Members(w http.ResponseWriter, r *http.Request) {
	g, err := h.svc.FindByName(r.URL.Query().Get("groupname"))
	if err != nil {
		v3.WriteError(w, http.StatusNotFound, []string{"group not found"}, nil)
		return
	}
	startAt, maxResults := v3.ParsePagination(r, 50, 100)
	ids, total, err := h.svc.MemberIDs(g.ID, startAt, maxResults)
	if err != nil {
		v3.WriteError(w, http.StatusInternalServerError, []string{"failed to list members"}, nil)
		return
	}
	values := make([]v3.User, 0, len(ids))
	for _, id := range ids {
		var u user.User
		if h.db.First(&u, "id = ?", id).Error == nil {
			values = append(values, v3.JiraUser(u, h.baseURL))
		}
	}
	v3.WritePage(w, http.StatusOK, v3.Page[v3.User]{StartAt: startAt, MaxResults: maxResults, Total: total, Values: values})
}

// AddUser: POST /rest/api/3/group/user?groupname=... {accountId} → 201 GroupRef.
func (h *GroupHandler) AddUser(w http.ResponseWriter, r *http.Request) {
	g, err := h.svc.FindByName(r.URL.Query().Get("groupname"))
	if err != nil {
		v3.WriteError(w, http.StatusNotFound, []string{"group not found"}, nil)
		return
	}
	var req struct {
		AccountID string `json:"accountId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.AccountID == "" {
		v3.WriteError(w, http.StatusBadRequest, []string{"accountId is required"}, nil)
		return
	}
	if err := h.svc.AddUser(g.ID, req.AccountID); err != nil {
		v3.WriteError(w, http.StatusInternalServerError, []string{"failed to add user"}, nil)
		return
	}
	v3.WriteJSON(w, http.StatusCreated, v3.JiraGroup(g.ID, g.Name, h.baseURL))
}

// RemoveUser: DELETE /rest/api/3/group/user?groupname=...&accountId=... → 200.
func (h *GroupHandler) RemoveUser(w http.ResponseWriter, r *http.Request) {
	g, err := h.svc.FindByName(r.URL.Query().Get("groupname"))
	if err != nil {
		v3.WriteError(w, http.StatusNotFound, []string{"group not found"}, nil)
		return
	}
	if err := h.svc.RemoveUser(g.ID, r.URL.Query().Get("accountId")); err != nil {
		v3.WriteError(w, http.StatusInternalServerError, []string{"failed to remove user"}, nil)
		return
	}
	w.WriteHeader(http.StatusOK)
}

// Picker: GET /rest/api/3/groups/picker?query=... → FoundGroups.
func (h *GroupHandler) Picker(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("query")
	groups, err := h.svc.Search(q, 20)
	if err != nil {
		v3.WriteError(w, http.StatusInternalServerError, []string{"failed to search groups"}, nil)
		return
	}
	out := v3.FoundGroups{
		Header: fmt.Sprintf("Showing %d of %d matching groups", len(groups), len(groups)),
		Total:  len(groups),
		Groups: make([]v3.FoundGroup, 0, len(groups)),
	}
	for _, g := range groups {
		out.Groups = append(out.Groups, v3.FoundGroup{Name: g.Name, GroupID: g.ID})
	}
	v3.WriteJSON(w, http.StatusOK, out)
}
