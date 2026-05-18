package handlers

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/open-jira/open-jira/internal/domain/issue"
	"github.com/open-jira/open-jira/internal/domain/project"
	"github.com/open-jira/open-jira/internal/domain/workflow"
)

type IssueHandler struct {
	svc        *issue.Service
	projectSvc *project.Service
	wfSvc      *workflow.Service
}

func NewIssueHandler(svc *issue.Service, projectSvc *project.Service, wfSvc *workflow.Service) *IssueHandler {
	return &IssueHandler{svc: svc, projectSvc: projectSvc, wfSvc: wfSvc}
}

func (h *IssueHandler) ExportCSV(w http.ResponseWriter, r *http.Request) {
	p, err := h.projectSvc.GetByKey(r.PathValue("key"))
	if err != nil {
		http.Error(w, `{"error":"project not found"}`, http.StatusNotFound)
		return
	}
	issues, _ := h.svc.ListByProject(p.ID)
	w.Header().Set("Content-Type", "text/csv")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s-issues.csv", p.Key))
	wr := csv.NewWriter(w)
	wr.Write([]string{"Key", "Title", "Priority", "Status", "Type", "Assignee", "Story Points", "Created", "Updated"})
	for _, iss := range issues {
		status := ""
		if iss.StatusID != nil {
			status = *iss.StatusID
		}
		typeName := ""
		if iss.TypeID != nil {
			typeName = *iss.TypeID
		}
		assignee := ""
		if iss.AssigneeID != nil {
			assignee = *iss.AssigneeID
		}
		wr.Write([]string{
			iss.Key,
			iss.Title,
			string(iss.Priority),
			status,
			typeName,
			assignee,
			fmt.Sprintf("%d", iss.StoryPoints),
			iss.CreatedAt.Format("2006-01-02"),
			iss.UpdatedAt.Format("2006-01-02"),
		})
	}
	wr.Flush()
}

func (h *IssueHandler) Create(w http.ResponseWriter, r *http.Request) {
	projectKey := r.PathValue("key")
	var req struct {
		ProjectKey  string         `json:"project_key"`
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
	if projectKey == "" {
		projectKey = req.ProjectKey
	}
	projectID := req.ProjectID
	if projectKey == "" && projectID == "" {
		http.Error(w, `{"error":"project_key or project_id is required"}`, http.StatusBadRequest)
		return
	}
	if projectID == "" {
		p, err := h.projectSvc.GetByKey(projectKey)
		if err != nil {
			http.Error(w, `{"error":"project not found"}`, http.StatusNotFound)
			return
		}
		projectID = p.ID
	}
	if projectKey == "" {
		p, err := h.projectSvc.GetByID(projectID)
		if err != nil {
			http.Error(w, `{"error":"project not found"}`, http.StatusNotFound)
			return
		}
		projectKey = p.Key
	}
	iss, err := h.svc.Create(projectKey, projectID, req.Title, req.Description, req.Priority, req.ParentID, req.TypeID)
	if err != nil {
		http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusInternalServerError)
		return
	}
	if h.wfSvc != nil {
		wf, wfErr := h.wfSvc.GetWorkflow(projectID)
		if wfErr == nil {
			for _, status := range wf.Statuses {
				if status.Category == workflow.CategoryTodo {
					iss, _ = h.svc.Update(iss.Key, nil, nil, nil, nil, &status.ID, nil)
					break
				}
			}
		}
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
	p, err := h.projectSvc.GetByKey(r.PathValue("key"))
	if err != nil {
		http.Error(w, `{"error":"project not found"}`, http.StatusNotFound)
		return
	}
	issues, _ := h.svc.ListByProject(p.ID)
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

func (h *IssueHandler) GetWatchers(w http.ResponseWriter, r *http.Request) {
	iss, err := h.svc.GetByKey(r.PathValue("issueIdOrKey"))
	if err != nil {
		http.Error(w, `{"error":"issue not found"}`, http.StatusNotFound)
		return
	}
	watchers, _ := h.svc.GetWatchers(iss.ID)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"watchers": watchers})
}

func (h *IssueHandler) AddWatcher(w http.ResponseWriter, r *http.Request) {
	iss, err := h.svc.GetByKey(r.PathValue("issueIdOrKey"))
	if err != nil {
		http.Error(w, `{"error":"issue not found"}`, http.StatusNotFound)
		return
	}
	var username string
	if err := json.NewDecoder(r.Body).Decode(&username); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}
	if username == "" {
		http.Error(w, `{"error":"username is required"}`, http.StatusBadRequest)
		return
	}
	var user struct {
		ID string `gorm:"column:id"`
	}
	if err := h.svc.DB().Table("users").Where("username = ?", username).Select("id").Scan(&user).Error; err != nil || user.ID == "" {
		http.Error(w, `{"error":"user not found"}`, http.StatusNotFound)
		return
	}
	if err := h.svc.Watch(iss.ID, user.ID); err != nil {
		http.Error(w, `{"error":"failed to add watcher"}`, http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *IssueHandler) RemoveWatcher(w http.ResponseWriter, r *http.Request) {
	iss, err := h.svc.GetByKey(r.PathValue("issueIdOrKey"))
	if err != nil {
		http.Error(w, `{"error":"issue not found"}`, http.StatusNotFound)
		return
	}
	username := r.URL.Query().Get("username")
	if username == "" {
		http.Error(w, `{"error":"username query parameter is required"}`, http.StatusBadRequest)
		return
	}
	var user struct {
		ID string `gorm:"column:id"`
	}
	if err := h.svc.DB().Table("users").Where("username = ?", username).Select("id").Scan(&user).Error; err != nil || user.ID == "" {
		http.Error(w, `{"error":"user not found"}`, http.StatusNotFound)
		return
	}
	h.svc.Unwatch(iss.ID, user.ID)
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
