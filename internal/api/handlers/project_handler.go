package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/it4nodummies/heureum/internal/api/middleware"
	v3 "github.com/it4nodummies/heureum/internal/api/v3"
	"github.com/it4nodummies/heureum/internal/domain/project"
	"github.com/it4nodummies/heureum/internal/domain/user"
	"github.com/it4nodummies/heureum/internal/domain/workflow"
)

type ProjectHandler struct {
	svc     *project.Service
	wfSvc   *workflow.Service
	baseURL string
}

func NewProjectHandler(svc *project.Service, wfSvc *workflow.Service, baseURL string) *ProjectHandler {
	return &ProjectHandler{svc: svc, wfSvc: wfSvc, baseURL: baseURL}
}

// leadOf resolves a project's lead user, returning nil if unset or not found.
func (h *ProjectHandler) leadOf(p *project.Project) *user.User {
	if p.LeadUserID == nil {
		return nil
	}
	var u user.User
	if err := h.svc.DB().First(&u, "id = ?", *p.LeadUserID).Error; err != nil {
		return nil
	}
	return &u
}

// categoryOf resolves a project's category, returning nil if unset or not found.
func (h *ProjectHandler) categoryOf(p *project.Project) *project.ProjectCategory {
	if p.CategoryID == nil {
		return nil
	}
	c, _ := h.svc.GetCategory(*p.CategoryID)
	return c
}

func (h *ProjectHandler) Create(w http.ResponseWriter, r *http.Request) {
	var raw struct {
		Key                string `json:"key"`
		Name               string `json:"name"`
		Description        string `json:"description"`
		ProjectTypeKey     string `json:"projectTypeKey"`
		ProjectTemplateKey string `json:"projectTemplateKey"`
		LeadAccountID      string `json:"leadAccountId"`
		AssigneeType       string `json:"assigneeType"`
		URL                string `json:"url"`
	}
	if err := json.NewDecoder(r.Body).Decode(&raw); err != nil {
		v3.WriteError(w, http.StatusBadRequest, []string{"Invalid request body."}, nil)
		return
	}
	fieldErrs := map[string]string{}
	if raw.Key == "" {
		fieldErrs["key"] = "You must specify a valid project key."
	}
	if raw.Name == "" {
		fieldErrs["name"] = "You must specify a valid project name."
	}
	if len(fieldErrs) > 0 {
		v3.WriteError(w, http.StatusBadRequest, nil, fieldErrs)
		return
	}
	in := project.CreateInput{
		Key:          raw.Key,
		Name:         raw.Name,
		Description:  raw.Description,
		Type:         project.TypeForTemplateKey(raw.ProjectTemplateKey),
		AssigneeType: raw.AssigneeType,
		URL:          raw.URL,
		CreatorID:    middleware.UserIDFromContext(r.Context()),
	}
	if raw.LeadAccountID != "" {
		in.LeadUserID = &raw.LeadAccountID
	}
	p, err := h.svc.CreateProject(in)
	if err != nil {
		if errors.Is(err, project.ErrInvalidKey) {
			v3.WriteError(w, http.StatusBadRequest, nil, map[string]string{"key": project.ErrInvalidKey.Error()})
			return
		}
		v3.WriteError(w, http.StatusBadRequest, []string{err.Error()}, nil)
		return
	}
	if _, err := h.wfSvc.CreateDefaultWorkflow(p.ID); err != nil {
		log.Printf("failed to create default workflow for project %s: %v", p.Key, err)
	}
	v3.WriteJSON(w, http.StatusCreated, map[string]any{
		"self": h.baseURL + "/rest/api/3/project/" + fmt.Sprint(p.SeqID),
		"id":   p.SeqID,
		"key":  p.Key,
	})
}

func (h *ProjectHandler) Get(w http.ResponseWriter, r *http.Request) {
	idOrKey := r.PathValue("key")
	// The path segment may be either a numeric project id (seq_id) or a
	// project key. Resolve numerically first when it's all digits.
	var (
		p   *project.Project
		err error
	)
	if n, convErr := strconv.ParseInt(idOrKey, 10, 64); convErr == nil {
		p, err = h.svc.GetBySeqID(n)
	} else {
		p, err = h.svc.GetByKey(idOrKey)
	}
	if err != nil || p == nil {
		v3.WriteError(w, http.StatusNotFound, []string{"No project could be found with key '" + idOrKey + "'."}, nil)
		return
	}
	v3.WriteJSON(w, http.StatusOK, v3.JiraProject(*p, h.leadOf(p), h.categoryOf(p), h.baseURL))
}

// resolveKey maps a {projectIdOrKey} path segment (numeric seq_id or project
// key) to the canonical project key, mirroring Get's lookup rule.
func (h *ProjectHandler) resolveKey(idOrKey string) (string, bool) {
	if n, err := strconv.ParseInt(idOrKey, 10, 64); err == nil {
		p, err := h.svc.GetBySeqID(n)
		if err != nil || p == nil {
			return "", false
		}
		return p.Key, true
	}
	p, err := h.svc.GetByKey(idOrKey)
	if err != nil || p == nil {
		return "", false
	}
	return p.Key, true
}

// List supports: query, type (comma-sep), orderBy, direction, startAt, maxResults
func (h *ProjectHandler) List(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	var types []string
	if tp := q.Get("type"); tp != "" {
		for _, t := range strings.Split(tp, ",") {
			if t = strings.TrimSpace(t); t != "" {
				types = append(types, t)
			}
		}
	}

	startAt, _ := strconv.Atoi(q.Get("startAt"))
	maxResults, _ := strconv.Atoi(q.Get("maxResults"))
	if maxResults == 0 {
		maxResults = 50
	}

	filter := project.ListFilter{
		Search:     q.Get("query"),
		Types:      types,
		SortKey:    q.Get("orderBy"),
		SortDir:    q.Get("direction"),
		StartAt:    startAt,
		MaxResults: maxResults,
	}

	userID := middleware.UserIDFromContext(r.Context())
	projects, total, err := h.svc.ListWithFilters(filter, userID)
	if err != nil {
		http.Error(w, `{"error":"failed to list projects"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"startAt":    startAt,
		"maxResults": maxResults,
		"total":      total,
		"isLast":     startAt+maxResults >= int(total),
		"values":     projects,
	})
}

// Search implements GET /rest/api/3/project/search, returning a
// PageBeanProject conformant to the official contract.
func (h *ProjectHandler) Search(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	startAt, maxResults := v3.ParsePagination(r, 50, 100)
	f := project.ListFilter{
		Search:     r.URL.Query().Get("query"),
		SortKey:    "name",
		SortDir:    "asc",
		StartAt:    startAt,
		MaxResults: maxResults,
	}
	rows, total, err := h.svc.ListWithFilters(f, userID)
	if err != nil {
		v3.WriteError(w, http.StatusInternalServerError, []string{"Failed to search projects."}, nil)
		return
	}
	values := make([]v3.Project, 0, len(rows))
	for i := range rows {
		var lead *user.User
		if rows[i].Lead != nil {
			lead = &user.User{
				ID:          rows[i].Lead.ID,
				DisplayName: rows[i].Lead.DisplayName,
				Email:       rows[i].Lead.Email,
				AvatarURL:   rows[i].Lead.AvatarURL,
				IsActive:    true,
			}
		}
		cat := h.categoryOf(&rows[i].Project)
		values = append(values, v3.JiraProject(rows[i].Project, lead, cat, h.baseURL))
	}
	v3.WritePage(w, http.StatusOK, v3.Page[v3.Project]{
		StartAt:    startAt,
		MaxResults: maxResults,
		Total:      int(total),
		Values:     values,
	})
}

func (h *ProjectHandler) Update(w http.ResponseWriter, r *http.Request) {
	idOrKey := r.PathValue("key")
	key, ok := h.resolveKey(idOrKey)
	if !ok {
		v3.WriteError(w, http.StatusNotFound, []string{"No project could be found with key or id '" + idOrKey + "'."}, nil)
		return
	}
	var req struct {
		Name         *string `json:"name"`
		Description  *string `json:"description"`
		AssigneeType *string `json:"assigneeType"`
		URL          *string `json:"url"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		v3.WriteError(w, http.StatusBadRequest, []string{"Invalid request body."}, nil)
		return
	}
	cur, err := h.svc.GetByKey(key)
	if err != nil || cur == nil {
		v3.WriteError(w, http.StatusNotFound, []string{"No project could be found with key '" + key + "'."}, nil)
		return
	}
	name, desc := cur.Name, cur.Description
	if req.Name != nil {
		name = *req.Name
	}
	if req.Description != nil {
		desc = *req.Description
	}
	if _, err := h.svc.Update(key, name, desc); err != nil {
		v3.WriteError(w, http.StatusNotFound, []string{"No project could be found with key '" + key + "'."}, nil)
		return
	}
	updates := map[string]any{}
	if req.AssigneeType != nil {
		updates["assignee_type"] = *req.AssigneeType
	}
	if req.URL != nil {
		updates["url"] = *req.URL
	}
	if len(updates) > 0 {
		h.svc.DB().Model(&project.Project{}).Where("key = ?", key).Updates(updates)
	}
	p, err := h.svc.GetByKey(key)
	if err != nil || p == nil {
		v3.WriteError(w, http.StatusNotFound, []string{"No project could be found with key '" + key + "'."}, nil)
		return
	}
	v3.WriteJSON(w, http.StatusOK, v3.JiraProject(*p, h.leadOf(p), h.categoryOf(p), h.baseURL))
}

// Archive implements POST /rest/api/3/project/{key}/archive: marks the
// project as archived and returns 204 with no body.
func (h *ProjectHandler) Archive(w http.ResponseWriter, r *http.Request) {
	idOrKey := r.PathValue("key")
	key, ok := h.resolveKey(idOrKey)
	if !ok {
		v3.WriteError(w, http.StatusNotFound, []string{"No project could be found with key or id '" + idOrKey + "'."}, nil)
		return
	}
	if err := h.svc.Archive(key); err != nil {
		v3.WriteError(w, http.StatusNotFound, []string{"No project could be found with key '" + key + "'."}, nil)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// Restore implements POST /rest/api/3/project/{key}/restore: clears the
// archived flag and returns the resulting Project.
func (h *ProjectHandler) Restore(w http.ResponseWriter, r *http.Request) {
	idOrKey := r.PathValue("key")
	key, ok := h.resolveKey(idOrKey)
	if !ok {
		v3.WriteError(w, http.StatusNotFound, []string{"No project could be found with key or id '" + idOrKey + "'."}, nil)
		return
	}
	if err := h.svc.Restore(key); err != nil {
		v3.WriteError(w, http.StatusNotFound, []string{"No project could be found with key '" + key + "'."}, nil)
		return
	}
	p, err := h.svc.GetByKey(key)
	if err != nil || p == nil {
		v3.WriteError(w, http.StatusNotFound, []string{"No project could be found with key '" + key + "'."}, nil)
		return
	}
	v3.WriteJSON(w, http.StatusOK, v3.JiraProject(*p, h.leadOf(p), h.categoryOf(p), h.baseURL))
}

func (h *ProjectHandler) Delete(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.Archive(r.PathValue("key")); err != nil {
		http.Error(w, `{"error":"project not found"}`, http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *ProjectHandler) ListMembers(w http.ResponseWriter, r *http.Request) {
	p, err := h.svc.GetByKey(r.PathValue("key"))
	if err != nil {
		http.Error(w, `{"error":"project not found"}`, http.StatusNotFound)
		return
	}
	members, _ := h.svc.ListMembers(p.ID)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(members)
}

func (h *ProjectHandler) AddMember(w http.ResponseWriter, r *http.Request) {
	p, err := h.svc.GetByKey(r.PathValue("key"))
	if err != nil {
		http.Error(w, `{"error":"project not found"}`, http.StatusNotFound)
		return
	}
	var req struct {
		UserID string             `json:"user_id"`
		Role   project.MemberRole `json:"role"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	if err := h.svc.AddMember(p.ID, req.UserID, req.Role); err != nil {
		http.Error(w, `{"error":"failed to add member"}`, http.StatusConflict)
		return
	}
	w.WriteHeader(http.StatusCreated)
}

func (h *ProjectHandler) RemoveMember(w http.ResponseWriter, r *http.Request) {
	p, err := h.svc.GetByKey(r.PathValue("key"))
	if err != nil {
		http.Error(w, `{"error":"project not found"}`, http.StatusNotFound)
		return
	}
	h.svc.RemoveMember(p.ID, r.PathValue("userId"))
	w.WriteHeader(http.StatusNoContent)
}

func (h *ProjectHandler) Invite(w http.ResponseWriter, r *http.Request) {
	p, err := h.svc.GetByKey(r.PathValue("key"))
	if err != nil {
		http.Error(w, `{"error":"project not found"}`, http.StatusNotFound)
		return
	}
	var req struct {
		Email string             `json:"email"`
		Role  project.MemberRole `json:"role"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	inv, err := project.CreateInvite(h.svc.DB(), p.ID, req.Email, req.Role)
	if err != nil {
		http.Error(w, `{"error":"failed to create invite"}`, http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{"token": inv.Token})
}

// StarProject adds a project to the current user's favourites.
func (h *ProjectHandler) StarProject(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	p, err := h.svc.GetByKey(r.PathValue("key"))
	if err != nil {
		http.Error(w, `{"error":"project not found"}`, http.StatusNotFound)
		return
	}
	if err := h.svc.Star(p.ID, userID); err != nil {
		http.Error(w, `{"error":"failed to star project"}`, http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// UnstarProject removes a project from the current user's favourites.
func (h *ProjectHandler) UnstarProject(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	p, err := h.svc.GetByKey(r.PathValue("key"))
	if err != nil {
		http.Error(w, `{"error":"project not found"}`, http.StatusNotFound)
		return
	}
	if err := h.svc.Unstar(p.ID, userID); err != nil {
		http.Error(w, `{"error":"failed to unstar project"}`, http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ProjectTypes implements GET /rest/api/3/project/type, listing the project
// types supported by this instance.
func (h *ProjectHandler) ProjectTypes(w http.ResponseWriter, r *http.Request) {
	types := []v3.ProjectType{
		v3.JiraProjectType("software", h.baseURL),
		v3.JiraProjectType("business", h.baseURL),
	}
	v3.WriteJSON(w, http.StatusOK, types)
}

// ProjectTypeByKey implements GET /rest/api/3/project/type/{projectTypeKey}.
func (h *ProjectHandler) ProjectTypeByKey(w http.ResponseWriter, r *http.Request) {
	key := r.PathValue("projectTypeKey")
	if key != "software" && key != "business" {
		v3.WriteError(w, http.StatusNotFound, []string{"No project type '" + key + "'."}, nil)
		return
	}
	v3.WriteJSON(w, http.StatusOK, v3.JiraProjectType(key, h.baseURL))
}
