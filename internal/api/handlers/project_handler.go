package handlers

import (
	"encoding/json"
	"hash/fnv"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/open-jira/open-jira/internal/api/middleware"
	v3 "github.com/open-jira/open-jira/internal/api/v3"
	"github.com/open-jira/open-jira/internal/domain/project"
	"github.com/open-jira/open-jira/internal/domain/user"
	"github.com/open-jira/open-jira/internal/domain/workflow"
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

// projectNumericID derives a stable int64 from a project's UUID. The Jira v3
// contract's ProjectIdentifiers schema declares "id" as an int64 (legacy
// Jira project IDs are numeric), while our internal storage keys projects by
// UUID. We hash the UUID into a positive int64 so the response conforms to
// the contract's type without needing a separate numeric column.
func projectNumericID(uuid string) int64 {
	hh := fnv.New64a()
	_, _ = hh.Write([]byte(uuid))
	return int64(hh.Sum64() &^ (1 << 63))
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
	}
	if raw.LeadAccountID != "" {
		in.LeadUserID = &raw.LeadAccountID
	}
	p, err := h.svc.CreateProject(in)
	if err != nil {
		v3.WriteError(w, http.StatusBadRequest, []string{err.Error()}, nil)
		return
	}
	if _, err := h.wfSvc.CreateDefaultWorkflow(p.ID); err != nil {
		log.Printf("failed to create default workflow for project %s: %v", p.Key, err)
	}
	v3.WriteJSON(w, http.StatusCreated, map[string]any{
		"self": h.baseURL + "/rest/api/3/project/" + p.ID,
		"id":   projectNumericID(p.ID),
		"key":  p.Key,
	})
}

func (h *ProjectHandler) Get(w http.ResponseWriter, r *http.Request) {
	key := r.PathValue("key")
	p, err := h.svc.GetByKey(key)
	if err != nil || p == nil {
		v3.WriteError(w, http.StatusNotFound, []string{"No project could be found with key '" + key + "'."}, nil)
		return
	}
	v3.WriteJSON(w, http.StatusOK, v3.JiraProject(*p, h.leadOf(p), h.categoryOf(p), h.baseURL))
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

func (h *ProjectHandler) Update(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	p, err := h.svc.Update(r.PathValue("key"), req.Name, req.Description)
	if err != nil {
		http.Error(w, `{"error":"project not found"}`, http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(p)
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
