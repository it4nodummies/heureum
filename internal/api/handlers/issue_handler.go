package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/open-jira/open-jira/internal/domain/issue"
)

type IssueHandler struct {
	svc *issue.Service
}

func NewIssueHandler(svc *issue.Service) *IssueHandler { return &IssueHandler{svc: svc} }

func (h *IssueHandler) Create(w http.ResponseWriter, r *http.Request) {
	projectKey := r.PathValue("key")
	var req struct {
		ProjectID   string         `json:"project_id"`
		Title       string         `json:"title"`
		Description string         `json:"description"`
		Priority    issue.Priority `json:"priority"`
		ParentID    *string        `json:"parent_id"`
		TypeID      *string        `json:"type_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}
	if req.Title == "" {
		http.Error(w, `{"error":"title is required"}`, http.StatusBadRequest)
		return
	}
	if req.Priority == "" {
		req.Priority = issue.PriorityMedium
	}
	iss, err := h.svc.Create(projectKey, req.ProjectID, req.Title, req.Description, req.Priority, req.ParentID, req.TypeID)
	if err != nil {
		http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(iss)
}

func (h *IssueHandler) Get(w http.ResponseWriter, r *http.Request) {
	iss, err := h.svc.GetByKey(r.PathValue("issueKey"))
	if err != nil {
		http.Error(w, `{"error":"issue not found"}`, http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(iss)
}

func (h *IssueHandler) Update(w http.ResponseWriter, r *http.Request) {
	issueKey := r.PathValue("issueKey")
	var req struct {
		Title           *string         `json:"title"`
		DescriptionJSON *string         `json:"description_json"`
		Priority        *issue.Priority `json:"priority"`
		AssigneeID      *string         `json:"assignee_id"`
		StatusID        *string         `json:"status_id"`
		StoryPoints     *int            `json:"story_points"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	iss, err := h.svc.Update(issueKey, req.Title, req.DescriptionJSON, req.Priority, req.AssigneeID, req.StatusID, req.StoryPoints)
	if err != nil {
		http.Error(w, `{"error":"issue not found"}`, http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(iss)
}

func (h *IssueHandler) Delete(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.Delete(r.PathValue("issueKey")); err != nil {
		http.Error(w, `{"error":"issue not found"}`, http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *IssueHandler) List(w http.ResponseWriter, r *http.Request) {
	projectKey := r.PathValue("key")
	issues, _ := h.svc.ListByProject(projectKey)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(issues)
}

func (h *IssueHandler) AddLabel(w http.ResponseWriter, r *http.Request) {
	iss, err := h.svc.GetByKey(r.PathValue("issueKey"))
	if err != nil {
		http.Error(w, `{"error":"issue not found"}`, http.StatusNotFound)
		return
	}
	var req struct {
		Name  string `json:"name"`
		Color string `json:"color"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	label, err := h.svc.AddLabel(iss.ID, iss.ProjectID, req.Name, req.Color)
	if err != nil {
		http.Error(w, `{"error":"failed to add label"}`, http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(label)
}

func (h *IssueHandler) ListLinks(w http.ResponseWriter, r *http.Request) {
	iss, err := h.svc.GetByKey(r.PathValue("issueKey"))
	if err != nil {
		http.Error(w, `{"error":"issue not found"}`, http.StatusNotFound)
		return
	}
	links, _ := h.svc.ListLinks(iss.ID)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(links)
}

func (h *IssueHandler) Watch(w http.ResponseWriter, r *http.Request) {
	iss, err := h.svc.GetByKey(r.PathValue("issueKey"))
	if err != nil {
		http.Error(w, `{"error":"issue not found"}`, http.StatusNotFound)
		return
	}
	var req struct {
		UserID string `json:"user_id"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	h.svc.Watch(iss.ID, req.UserID)
	w.WriteHeader(http.StatusCreated)
}

func (h *IssueHandler) Unwatch(w http.ResponseWriter, r *http.Request) {
	iss, err := h.svc.GetByKey(r.PathValue("issueKey"))
	if err != nil {
		http.Error(w, `{"error":"issue not found"}`, http.StatusNotFound)
		return
	}
	h.svc.Unwatch(iss.ID, r.URL.Query().Get("user_id"))
	w.WriteHeader(http.StatusNoContent)
}

func (h *IssueHandler) GetHistory(w http.ResponseWriter, r *http.Request) {
	iss, err := h.svc.GetByKey(r.PathValue("issueKey"))
	if err != nil {
		http.Error(w, `{"error":"issue not found"}`, http.StatusNotFound)
		return
	}
	history, _ := h.svc.GetHistory(iss.ID)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(history)
}
