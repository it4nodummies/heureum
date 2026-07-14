package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/it4nodummies/heureum/internal/api/v3"
	"github.com/it4nodummies/heureum/internal/domain/issue"
	"github.com/it4nodummies/heureum/internal/domain/project"
	"github.com/it4nodummies/heureum/internal/domain/workflow"
)

type WorkflowHandler struct {
	wfSvc      *workflow.Service
	issueSvc   *issue.Service
	projectSvc *project.Service
	baseURL    string
}

func NewWorkflowHandler(wfSvc *workflow.Service, issueSvc *issue.Service, projectSvc *project.Service, baseURL string) *WorkflowHandler {
	return &WorkflowHandler{wfSvc: wfSvc, issueSvc: issueSvc, projectSvc: projectSvc, baseURL: baseURL}
}

func (h *WorkflowHandler) GetWorkflow(w http.ResponseWriter, r *http.Request) {
	p, err := h.projectSvc.GetByKey(r.PathValue("key"))
	if err != nil {
		http.Error(w, `{"error":"project not found"}`, http.StatusNotFound)
		return
	}
	wf, err := h.wfSvc.GetWorkflow(p.ID)
	if err != nil {
		http.Error(w, `{"error":"workflow not found"}`, http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(wf)
}

func (h *WorkflowHandler) AddStatus(w http.ResponseWriter, r *http.Request) {
	p, err := h.projectSvc.GetByKey(r.PathValue("key"))
	if err != nil {
		http.Error(w, `{"error":"project not found"}`, http.StatusNotFound)
		return
	}
	wf, err := h.wfSvc.GetWorkflow(p.ID)
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
	p, err := h.projectSvc.GetByKey(r.PathValue("key"))
	if err != nil {
		http.Error(w, `{"error":"project not found"}`, http.StatusNotFound)
		return
	}
	wf, err := h.wfSvc.GetWorkflow(p.ID)
	if err != nil {
		http.Error(w, `{"error":"workflow not found"}`, http.StatusNotFound)
		return
	}
	var req struct {
		FromStatusID    string `json:"from_status_id"`
		ToStatusID      string `json:"to_status_id"`
		Name            string `json:"name"`
		RequireAssignee bool   `json:"require_assignee"`
		SetResolution   bool   `json:"set_resolution"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}
	tr, err := h.wfSvc.AddTransition(wf.ID, req.FromStatusID, req.ToStatusID, req.Name, req.RequireAssignee, req.SetResolution)
	if err != nil {
		http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(tr)
}

// ListTransitions gestisce GET /rest/api/3/project/{key}/workflow/transitions.
func (h *WorkflowHandler) ListTransitions(w http.ResponseWriter, r *http.Request) {
	p, err := h.projectSvc.GetByKey(r.PathValue("key"))
	if err != nil {
		v3.WriteError(w, http.StatusNotFound, []string{"project not found"}, nil)
		return
	}
	wf, err := h.wfSvc.GetWorkflow(p.ID)
	if err != nil {
		v3.WriteError(w, http.StatusNotFound, []string{"workflow not found"}, nil)
		return
	}
	trs, err := h.wfSvc.GetTransitions(wf.ID)
	if err != nil {
		v3.WriteError(w, http.StatusInternalServerError, []string{"failed to list transitions"}, nil)
		return
	}
	v3.WriteJSON(w, http.StatusOK, trs)
}

// UpdateTransition gestisce PATCH /rest/api/3/project/{key}/workflow/transitions/{id}.
func (h *WorkflowHandler) UpdateTransition(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name            *string `json:"name"`
		RequireAssignee *bool   `json:"require_assignee"`
		SetResolution   *bool   `json:"set_resolution"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		v3.WriteError(w, http.StatusBadRequest, []string{"invalid request body"}, nil)
		return
	}
	tr, err := h.wfSvc.UpdateTransition(r.PathValue("id"), req.Name, req.RequireAssignee, req.SetResolution)
	if err != nil {
		v3.WriteError(w, http.StatusNotFound, []string{"transition not found"}, nil)
		return
	}
	v3.WriteJSON(w, http.StatusOK, tr)
}

// DeleteTransition gestisce DELETE /rest/api/3/project/{key}/workflow/transitions/{id}.
func (h *WorkflowHandler) DeleteTransition(w http.ResponseWriter, r *http.Request) {
	if err := h.wfSvc.RemoveTransition(r.PathValue("id")); err != nil {
		v3.WriteError(w, http.StatusInternalServerError, []string{"failed to delete transition"}, nil)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ReorderStatuses gestisce PUT /rest/api/3/project/{key}/workflow/statuses/order.
func (h *WorkflowHandler) ReorderStatuses(w http.ResponseWriter, r *http.Request) {
	p, err := h.projectSvc.GetByKey(r.PathValue("key"))
	if err != nil {
		v3.WriteError(w, http.StatusNotFound, []string{"project not found"}, nil)
		return
	}
	wf, err := h.wfSvc.GetWorkflow(p.ID)
	if err != nil {
		v3.WriteError(w, http.StatusNotFound, []string{"workflow not found"}, nil)
		return
	}
	var req struct {
		StatusIDs []string `json:"status_ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		v3.WriteError(w, http.StatusBadRequest, []string{"invalid request body"}, nil)
		return
	}
	if err := h.wfSvc.ReorderStatuses(wf.ID, req.StatusIDs); err != nil {
		v3.WriteError(w, http.StatusInternalServerError, []string{"failed to reorder"}, nil)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// issueAndWorkflow trova la issue e il suo workflow.
func (h *WorkflowHandler) issueAndWorkflow(issueKey string) (*issue.Issue, *workflow.Workflow, error) {
	iss, err := h.issueSvc.GetByKey(issueKey)
	if err != nil {
		return nil, nil, err
	}
	wf, err := h.wfSvc.GetWorkflow(iss.ProjectID)
	if err != nil {
		return nil, nil, err
	}
	return iss, wf, nil
}

// statusByID cerca uno stato nel workflow.
func statusByID(wf *workflow.Workflow, id string) *workflow.WorkflowStatus {
	for i := range wf.Statuses {
		if wf.Statuses[i].ID == id {
			return &wf.Statuses[i]
		}
	}
	return nil
}

// AvailableTransitions gestisce GET /rest/api/3/issue/{issueKey}/transitions.
func (h *WorkflowHandler) AvailableTransitions(w http.ResponseWriter, r *http.Request) {
	iss, wf, err := h.issueAndWorkflow(r.PathValue("issueKey"))
	if err != nil {
		v3.WriteError(w, http.StatusNotFound, []string{"issue or workflow not found"}, nil)
		return
	}
	from := ""
	if iss.StatusID != nil {
		from = *iss.StatusID
	}
	trs, err := h.wfSvc.GetAvailableTransitions(wf.ID, from)
	if err != nil {
		v3.WriteError(w, http.StatusInternalServerError, []string{"failed to list transitions"}, nil)
		return
	}
	out := make([]v3.IssueTransition, 0, len(trs))
	for _, tr := range trs {
		to := statusByID(wf, tr.ToStatusID)
		if to == nil {
			continue
		}
		name := tr.Name
		if name == "" {
			name = "→ " + to.Name
		}
		out = append(out, v3.MakeTransition(v3.TransitionInput{
			ID: tr.ID, Name: name, ToID: to.ID, ToName: to.Name,
			ToCategory: string(to.Category), Available: true, BaseURL: h.baseURL,
		}))
	}
	v3.WriteJSON(w, http.StatusOK, v3.Transitions{Transitions: out})
}

// DoTransition gestisce POST /rest/api/3/issue/{issueKey}/transitions.
// Accetta lo shape Jira {transition:{id}} e l'estensione {status_id} (usata dalla board).
func (h *WorkflowHandler) DoTransition(w http.ResponseWriter, r *http.Request) {
	iss, wf, err := h.issueAndWorkflow(r.PathValue("issueKey"))
	if err != nil {
		v3.WriteError(w, http.StatusNotFound, []string{"issue or workflow not found"}, nil)
		return
	}
	var req struct {
		Transition struct {
			ID string `json:"id"`
		} `json:"transition"`
		StatusID string `json:"status_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		v3.WriteError(w, http.StatusBadRequest, []string{"invalid request body"}, nil)
		return
	}
	from := ""
	if iss.StatusID != nil {
		from = *iss.StatusID
	}

	// Risolvi la transizione target.
	var tr *workflow.WorkflowTransition
	switch {
	case req.Transition.ID != "":
		t, err := h.wfSvc.GetTransitionByID(req.Transition.ID)
		if err != nil || t.WorkflowID != wf.ID || t.FromStatusID != from {
			v3.WriteError(w, http.StatusBadRequest, []string{"invalid transition"}, nil)
			return
		}
		tr = t
	case req.StatusID != "":
		avail, _ := h.wfSvc.GetAvailableTransitions(wf.ID, from)
		for i := range avail {
			if avail[i].ToStatusID == req.StatusID {
				tr = &avail[i]
				break
			}
		}
		if tr == nil {
			v3.WriteError(w, http.StatusBadRequest, []string{"invalid transition"}, nil)
			return
		}
	default:
		v3.WriteError(w, http.StatusBadRequest, []string{"transition.id or status_id is required"}, nil)
		return
	}

	// Validator: require_assignee.
	if tr.RequireAssignee && (iss.AssigneeID == nil || *iss.AssigneeID == "") {
		v3.WriteError(w, http.StatusBadRequest, []string{"assignee is required for this transition"}, nil)
		return
	}

	// Applica il cambio di stato.
	if _, err := h.issueSvc.Update(iss.Key, nil, nil, nil, nil, &tr.ToStatusID, nil); err != nil {
		v3.WriteError(w, http.StatusInternalServerError, []string{"failed to update issue"}, nil)
		return
	}

	// Post-function: set_resolution in base alla categoria dello stato destinazione.
	if tr.SetResolution {
		toStatus := statusByID(wf, tr.ToStatusID)
		if toStatus != nil && toStatus.Category == workflow.CategoryDone {
			if resID, ok := h.issueSvc.ResolutionIDByName("Done"); ok {
				_ = h.issueSvc.SetResolution(iss.Key, &resID)
			}
		} else {
			_ = h.issueSvc.SetResolution(iss.Key, nil)
		}
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *WorkflowHandler) ListStatuses(w http.ResponseWriter, r *http.Request) {
	statuses, err := h.wfSvc.ListAllStatuses()
	if err != nil {
		http.Error(w, `{"error":"failed to list statuses"}`, http.StatusInternalServerError)
		return
	}
	if statuses == nil {
		statuses = []workflow.WorkflowStatus{}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(statuses)
}

func (h *WorkflowHandler) GetStatus(w http.ResponseWriter, r *http.Request) {
	status, err := h.wfSvc.GetStatus(r.PathValue("idOrName"))
	if err != nil {
		http.Error(w, `{"error":"status not found"}`, http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(status)
}

func (h *WorkflowHandler) SearchWorkflows(w http.ResponseWriter, r *http.Request) {
	projectID := r.URL.Query().Get("projectId")
	if projectID == "" {
		http.Error(w, `{"error":"projectId is required"}`, http.StatusBadRequest)
		return
	}
	wf, err := h.wfSvc.GetWorkflowByProjectID(projectID)
	if err != nil {
		http.Error(w, `{"error":"workflow not found"}`, http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode([]workflow.Workflow{*wf})
}
