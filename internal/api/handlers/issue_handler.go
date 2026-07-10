package handlers

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	v3 "github.com/open-jira/open-jira/internal/api/v3"
	"github.com/open-jira/open-jira/internal/domain/issue"
	"github.com/open-jira/open-jira/internal/domain/project"
	"github.com/open-jira/open-jira/internal/domain/user"
	"github.com/open-jira/open-jira/internal/domain/workflow"
)

type IssueHandler struct {
	svc        *issue.Service
	projectSvc *project.Service
	wfSvc      *workflow.Service
	baseURL    string
}

func NewIssueHandler(svc *issue.Service, projectSvc *project.Service, wfSvc *workflow.Service, baseURL string) *IssueHandler {
	return &IssueHandler{svc: svc, projectSvc: projectSvc, wfSvc: wfSvc, baseURL: baseURL}
}

// resolveIssue trova un'issue per SeqID numerico (id v3) o per Key.
func (h *IssueHandler) resolveIssue(idOrKey string) (*issue.Issue, error) {
	if n, err := strconv.ParseInt(idOrKey, 10, 64); err == nil {
		return h.svc.GetBySeqID(n)
	}
	return h.svc.GetByKey(idOrKey)
}

func itoaInt64(n int64) string { return strconv.FormatInt(n, 10) }

// buildIssueInput arricchisce un'issue di dominio con le entità collegate
// (assignee, reporter, tipo, stato, progetto, parent, label) necessarie a
// costruire un v3.IssueBean completo tramite v3.JiraIssue.
func (h *IssueHandler) buildIssueInput(iss *issue.Issue) v3.IssueInput {
	in := v3.IssueInput{Issue: *iss, BaseURL: h.baseURL}
	db := h.svc.DB()
	if iss.AssigneeID != nil {
		var u user.User
		if db.First(&u, "id = ?", *iss.AssigneeID).Error == nil {
			in.Assignee = &u
		}
	}
	if iss.ReporterID != nil {
		var u user.User
		if db.First(&u, "id = ?", *iss.ReporterID).Error == nil {
			in.Reporter = &u
		}
	}
	if iss.TypeID != nil {
		var it issue.IssueType
		if db.First(&it, "id = ?", *iss.TypeID).Error == nil {
			in.IssueType = &it
		}
	}
	if iss.StatusID != nil {
		var st workflow.WorkflowStatus
		if db.First(&st, "id = ?", *iss.StatusID).Error == nil {
			s := v3.JiraStatus(st.ID, st.Name, string(st.Category), h.baseURL)
			in.Status = &s
		}
	}
	var p project.Project
	if db.First(&p, "id = ?", iss.ProjectID).Error == nil {
		in.Project = &v3.ProjectRef{Self: h.baseURL + "/rest/api/3/project/" + itoaInt64(p.SeqID), ID: itoaInt64(p.SeqID), Key: p.Key, Name: p.Name}
	}
	if iss.ParentID != nil {
		var parent issue.Issue
		if db.First(&parent, "id = ?", *iss.ParentID).Error == nil {
			in.Parent = &v3.ParentRef{ID: itoaInt64(parent.SeqID), Key: parent.Key, Self: h.baseURL + "/rest/api/3/issue/" + itoaInt64(parent.SeqID)}
		}
	}
	labels, _ := h.svc.GetLabels(iss.ID)
	in.Labels = labels
	return in
}

// priorityEnumForID mappa gli ID di priorità Jira standard (1..5) all'enum
// interno usato da issue.Priority.
func priorityEnumForID(id string) string {
	return map[string]string{"1": "highest", "2": "high", "3": "medium", "4": "low", "5": "lowest"}[id]
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

// Create implementa POST /rest/api/3/issue: accetta il body Jira ufficiale
// {"fields": {...}} e restituisce 201 con lo shape CreatedIssue.
func (h *IssueHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Fields struct {
			Project struct {
				Key string `json:"key"`
				ID  string `json:"id"`
			} `json:"project"`
			Summary     string `json:"summary"`
			Description any    `json:"description"`
			IssueType   struct {
				ID   string `json:"id"`
				Name string `json:"name"`
			} `json:"issuetype"`
			Priority struct {
				ID string `json:"id"`
			} `json:"priority"`
			Parent struct {
				Key string `json:"key"`
			} `json:"parent"`
			Labels []string `json:"labels"`
		} `json:"fields"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		v3.WriteError(w, http.StatusBadRequest, []string{"Invalid request body."}, nil)
		return
	}

	fe := map[string]string{}
	if req.Fields.Summary == "" {
		fe["summary"] = "You must specify a summary of the issue."
	}
	if req.Fields.Project.Key == "" && req.Fields.Project.ID == "" {
		fe["project"] = "You must specify a valid project."
	}
	if len(fe) > 0 {
		v3.WriteError(w, http.StatusBadRequest, nil, fe)
		return
	}

	var proj *project.Project
	var err error
	if req.Fields.Project.Key != "" {
		proj, err = h.projectSvc.GetByKey(req.Fields.Project.Key)
	} else {
		proj, err = h.projectSvc.GetByID(req.Fields.Project.ID)
	}
	if err != nil || proj == nil {
		v3.WriteError(w, http.StatusBadRequest, nil, map[string]string{"project": "The project does not exist."})
		return
	}

	descJSON := ""
	if req.Fields.Description != nil {
		if b, err := json.Marshal(req.Fields.Description); err == nil {
			descJSON = string(b)
		}
	}
	priority := issue.PriorityMedium
	if e := priorityEnumForID(req.Fields.Priority.ID); e != "" {
		priority = issue.Priority(e)
	}
	var parentID *string
	if req.Fields.Parent.Key != "" {
		if parent, perr := h.svc.GetByKey(req.Fields.Parent.Key); perr == nil && parent != nil {
			parentID = &parent.ID
		}
	}
	var typeID *string
	if req.Fields.IssueType.ID != "" {
		typeID = &req.Fields.IssueType.ID
	}

	iss, err := h.svc.Create(proj.Key, proj.ID, req.Fields.Summary, descJSON, priority, parentID, typeID)
	if err != nil {
		v3.WriteError(w, http.StatusBadRequest, []string{err.Error()}, nil)
		return
	}
	// Mantiene l'auto-assegnazione dello status "todo" dal workflow del progetto.
	if h.wfSvc != nil {
		if wf, wfErr := h.wfSvc.GetWorkflow(proj.ID); wfErr == nil {
			for _, status := range wf.Statuses {
				if status.Category == workflow.CategoryTodo {
					iss, _ = h.svc.Update(iss.Key, nil, nil, nil, nil, &status.ID, nil)
					break
				}
			}
		}
	}
	for _, name := range req.Fields.Labels {
		_, _ = h.svc.AddLabel(iss.ID, proj.ID, name, "")
	}

	v3.WriteJSON(w, http.StatusCreated, map[string]any{
		"id":   itoaInt64(iss.SeqID),
		"key":  iss.Key,
		"self": h.baseURL + "/rest/api/3/issue/" + itoaInt64(iss.SeqID),
	})
}

// Get implementa GET /rest/api/3/issue/{issueKey} restituendo l'IssueBean v3
// ufficiale. Accetta sia la Key (es. DEMO-1) sia il SeqID numerico.
func (h *IssueHandler) Get(w http.ResponseWriter, r *http.Request) {
	iss, err := h.resolveIssue(r.PathValue("issueKey"))
	if err != nil || iss == nil {
		v3.WriteError(w, http.StatusNotFound, []string{"Issue does not exist or you do not have permission to see it."}, nil)
		return
	}
	// IssueFields usa omitempty sui campi pointer nilable, quindi i valori null
	// (assignee/reporter/resolution/description non impostati) vengono omessi:
	// lo schema di "fields" (additionalProperties:{} senza nullable) accetta la
	// chiave omessa ma non un null esplicito.
	v3.WriteJSON(w, http.StatusOK, v3.JiraIssue(h.buildIssueInput(iss)))
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
