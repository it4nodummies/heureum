package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/open-jira/open-jira/internal/domain/issue"
	"github.com/open-jira/open-jira/internal/domain/workflow"
)

type WorkflowHandler struct {
	wfSvc    *workflow.Service
	issueSvc *issue.Service
}

func NewWorkflowHandler(wfSvc *workflow.Service, issueSvc *issue.Service) *WorkflowHandler {
	return &WorkflowHandler{wfSvc: wfSvc, issueSvc: issueSvc}
}

func (h *WorkflowHandler) GetWorkflow(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("key")
	wf, err := h.wfSvc.GetWorkflow(projectID)
	if err != nil {
		http.Error(w, `{"error":"workflow not found"}`, http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(wf)
}

func (h *WorkflowHandler) AddStatus(w http.ResponseWriter, r *http.Request) {
	wf, err := h.wfSvc.GetWorkflow(r.PathValue("key"))
	if err != nil {
		http.Error(w, `{"error":"workflow not found"}`, http.StatusNotFound)
		return
	}
	var req struct {
		Name     string                  `json:"name"`
		Category workflow.StatusCategory `json:"category"`
		Color    string                  `json:"color"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}
	status, err := h.wfSvc.AddStatus(wf.ID, req.Name, req.Category, req.Color)
	if err != nil {
		http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(status)
}

func (h *WorkflowHandler) UpdateStatus(w http.ResponseWriter, r *http.Request) {
	statusID := r.PathValue("id")
	var req struct {
		Name     string                  `json:"name"`
		Category workflow.StatusCategory `json:"category"`
		Color    string                  `json:"color"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}
	status, err := h.wfSvc.UpdateStatus(statusID, req.Name, req.Category, req.Color)
	if err != nil {
		http.Error(w, `{"error":"status not found"}`, http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(status)
}

func (h *WorkflowHandler) DeleteStatus(w http.ResponseWriter, r *http.Request) {
	if err := h.wfSvc.RemoveStatus(r.PathValue("id")); err != nil {
		http.Error(w, `{"error":"failed to remove status"}`, http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *WorkflowHandler) AddTransition(w http.ResponseWriter, r *http.Request) {
	wf, err := h.wfSvc.GetWorkflow(r.PathValue("key"))
	if err != nil {
		http.Error(w, `{"error":"workflow not found"}`, http.StatusNotFound)
		return
	}
	var req struct {
		FromStatusID string `json:"from_status_id"`
		ToStatusID   string `json:"to_status_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}
	tr, err := h.wfSvc.AddTransition(wf.ID, req.FromStatusID, req.ToStatusID)
	if err != nil {
		http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(tr)
}

func (h *WorkflowHandler) TransitionIssue(w http.ResponseWriter, r *http.Request) {
	iss, err := h.issueSvc.GetByKey(r.PathValue("issueKey"))
	if err != nil {
		http.Error(w, `{"error":"issue not found"}`, http.StatusNotFound)
		return
	}
	var req struct {
		ToStatusID string `json:"to_status_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}
	fromStatusID := ""
	if iss.StatusID != nil {
		fromStatusID = *iss.StatusID
	}
	wf, err := h.wfSvc.GetWorkflow(iss.ProjectID)
	if err != nil {
		http.Error(w, `{"error":"workflow not found"}`, http.StatusNotFound)
		return
	}
	if err := h.wfSvc.ValidateTransition(wf.ID, fromStatusID, req.ToStatusID); err != nil {
		http.Error(w, `{"error":"invalid transition"}`, http.StatusBadRequest)
		return
	}
	updated, err := h.issueSvc.Update(iss.Key, nil, nil, nil, nil, &req.ToStatusID, nil)
	if err != nil {
		http.Error(w, `{"error":"failed to update issue"}`, http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(updated)
}
