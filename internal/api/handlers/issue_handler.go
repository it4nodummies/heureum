package handlers

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/it4nodummies/heureum/internal/api/authz"
	"github.com/it4nodummies/heureum/internal/api/middleware"
	v3 "github.com/it4nodummies/heureum/internal/api/v3"
	"github.com/it4nodummies/heureum/internal/domain/issue"
	"github.com/it4nodummies/heureum/internal/domain/permission"
	"github.com/it4nodummies/heureum/internal/domain/project"
	"github.com/it4nodummies/heureum/internal/domain/user"
	"github.com/it4nodummies/heureum/internal/domain/workflow"
)

type IssueHandler struct {
	svc        *issue.Service
	projectSvc *project.Service
	wfSvc      *workflow.Service
	chk        *authz.Checker
	baseURL    string
}

func NewIssueHandler(svc *issue.Service, projectSvc *project.Service, wfSvc *workflow.Service, chk *authz.Checker, baseURL string) *IssueHandler {
	return &IssueHandler{svc: svc, projectSvc: projectSvc, wfSvc: wfSvc, chk: chk, baseURL: baseURL}
}

// resolveIssue trova un'issue per SeqID numerico (id v3) o per Key. Le issue
// archiviate (soft-deleted da Delete) sono trattate come inesistenti.
func (h *IssueHandler) resolveIssue(idOrKey string) (*issue.Issue, error) {
	var iss *issue.Issue
	var err error
	if n, parseErr := strconv.ParseInt(idOrKey, 10, 64); parseErr == nil {
		iss, err = h.svc.GetBySeqID(n)
	} else {
		iss, err = h.svc.GetByKey(idOrKey)
	}
	if err != nil {
		return nil, err
	}
	if iss != nil && iss.IsArchived {
		return nil, nil
	}
	return iss, nil
}

func itoaInt64(n int64) string { return strconv.FormatInt(n, 10) }

// isEpic verifica se la issue è di tipo "Epic" (case-insensitive) risolvendo il type.
func (h *IssueHandler) isEpic(iss *issue.Issue) bool {
	if iss.TypeID == nil {
		return false
	}
	var it issue.IssueType
	if h.svc.DB().First(&it, "id = ?", *iss.TypeID).Error != nil {
		return false
	}
	return strings.EqualFold(it.Name, "Epic")
}

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
	if iss.ResolutionID != nil {
		var row struct {
			ID          string
			Name        string
			Description string
		}
		if db.Table("resolutions").Where("id = ?", *iss.ResolutionID).Scan(&row).Error == nil && row.ID != "" {
			r := v3.JiraResolution(row.ID, row.Name, row.Description, h.baseURL)
			in.Resolution = &r
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

	uid := middleware.UserIDFromContext(r.Context())
	if err := h.chk.RequireProject(uid, proj.ID, permission.CreateIssues); err != nil {
		authz.WriteForbidden(w)
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
	} else if req.Fields.IssueType.Name != "" {
		// La UI (e Jira reale) manda tipicamente issuetype.name, non .id: senza
		// questa risoluzione l'issue veniva creata con TypeID nil e la v3
		// mapping layer applicava un fallback "Task" fisso (v3/issue.go),
		// mascherando silenziosamente qualunque altro tipo richiesto — incluso
		// "Subtask" per le sottotask create dalla sezione Subtasks.
		if id, terr := h.svc.TypeIDByName(proj.ID, req.Fields.IssueType.Name); terr == nil {
			typeID = &id
		}
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

// Update implementa PUT /rest/api/3/issue/{issueIdOrKey}: accetta il body
// Jira ufficiale {"fields": {...}} e restituisce 204 senza corpo.
func (h *IssueHandler) Update(w http.ResponseWriter, r *http.Request) {
	iss, err := h.resolveIssue(r.PathValue("issueKey"))
	if err != nil || iss == nil {
		v3.WriteError(w, http.StatusNotFound, []string{"Issue does not exist or you do not have permission to see it."}, nil)
		return
	}
	var req struct {
		Fields struct {
			Summary     *string  `json:"summary"`
			Description any      `json:"description"`
			Labels      []string `json:"labels"`
			Assignee    *struct {
				AccountID string `json:"accountId"`
			} `json:"assignee"`
			Priority *struct {
				ID string `json:"id"`
			} `json:"priority"`
			StoryPoints  *int `json:"customfield_10016"`
			TimeTracking *struct {
				OriginalEstimateSeconds  *int `json:"originalEstimateSeconds"`
				RemainingEstimateSeconds *int `json:"remainingEstimateSeconds"`
			} `json:"timetracking"`
		} `json:"fields"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		v3.WriteError(w, http.StatusBadRequest, []string{"Invalid request body."}, nil)
		return
	}
	var title, descJSON *string
	if req.Fields.Summary != nil {
		title = req.Fields.Summary
	}
	if req.Fields.Description != nil {
		if b, err := json.Marshal(req.Fields.Description); err == nil {
			s := string(b)
			descJSON = &s
		}
	}
	var priority *issue.Priority
	if req.Fields.Priority != nil {
		if e := priorityEnumForID(req.Fields.Priority.ID); e != "" {
			p := issue.Priority(e)
			priority = &p
		}
	}
	var assigneeID *string
	if req.Fields.Assignee != nil {
		assigneeID = &req.Fields.Assignee.AccountID
	}
	if _, err := h.svc.Update(iss.Key, title, descJSON, priority, assigneeID, nil, req.Fields.StoryPoints); err != nil {
		v3.WriteError(w, http.StatusInternalServerError, []string{"Failed to update issue."}, nil)
		return
	}
	if req.Fields.Labels != nil {
		if err := h.svc.SetLabels(iss.ID, iss.ProjectID, req.Fields.Labels); err != nil {
			v3.WriteError(w, http.StatusInternalServerError, []string{"Failed to update labels."}, nil)
			return
		}
	}
	if req.Fields.TimeTracking != nil {
		if _, err := h.svc.SetTimeTracking(iss.Key, req.Fields.TimeTracking.OriginalEstimateSeconds, req.Fields.TimeTracking.RemainingEstimateSeconds); err != nil {
			v3.WriteError(w, http.StatusInternalServerError, []string{"Failed to update time tracking."}, nil)
			return
		}
	}
	w.WriteHeader(http.StatusNoContent)
}

// Delete implementa DELETE /rest/api/3/issue/{issueIdOrKey} restituendo 204
// senza corpo, o 404 in stile Jira se l'issue non esiste.
func (h *IssueHandler) Delete(w http.ResponseWriter, r *http.Request) {
	iss, err := h.resolveIssue(r.PathValue("issueKey"))
	if err != nil || iss == nil {
		v3.WriteError(w, http.StatusNotFound, []string{"Issue does not exist or you do not have permission to see it."}, nil)
		return
	}
	if err := h.svc.Delete(iss.Key); err != nil {
		v3.WriteError(w, http.StatusInternalServerError, []string{"Failed to delete issue."}, nil)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// Subtasks implementa GET /rest/api/3/issue/{issueIdOrKey}/subtasks: elenca i
// figli (issue.parent_id == issue.ID) come v3 issue completi, riusando la
// stessa costruzione (buildIssueInput + v3.JiraIssue) di Get.
func (h *IssueHandler) Subtasks(w http.ResponseWriter, r *http.Request) {
	iss, err := h.resolveIssue(r.PathValue("issueIdOrKey"))
	if err != nil || iss == nil {
		v3.WriteError(w, http.StatusNotFound, []string{"Issue does not exist or you do not have permission to see it."}, nil)
		return
	}
	children, err := h.svc.GetChildren(iss.ID)
	if err != nil {
		v3.WriteError(w, http.StatusInternalServerError, []string{"failed to list subtasks"}, nil)
		return
	}
	out := make([]v3.IssueBean, 0, len(children))
	for i := range children {
		out = append(out, v3.JiraIssue(h.buildIssueInput(&children[i])))
	}
	v3.WriteJSON(w, http.StatusOK, map[string]any{"values": out, "total": len(out)})
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

// GetWatchers implementa GET /rest/api/3/issue/{issueIdOrKey}/watchers
// restituendo lo schema Watchers v3 (self, isWatching, watchCount, watchers).
func (h *IssueHandler) GetWatchers(w http.ResponseWriter, r *http.Request) {
	iss, err := h.resolveIssue(r.PathValue("issueIdOrKey"))
	if err != nil || iss == nil {
		v3.WriteError(w, http.StatusNotFound, []string{"Issue does not exist or you do not have permission to see it."}, nil)
		return
	}
	wl, _ := h.svc.GetWatchers(iss.ID)
	ids := make([]string, 0, len(wl))
	for _, x := range wl {
		ids = append(ids, x.UserID)
	}
	var users []user.User
	if len(ids) > 0 {
		h.svc.DB().Where("id IN ?", ids).Find(&users)
	}
	current := middleware.UserIDFromContext(r.Context())
	isWatching := false
	for _, id := range ids {
		if id == current {
			isWatching = true
			break
		}
	}
	v3.WriteJSON(w, http.StatusOK, v3.JiraWatchers(iss.Key, h.baseURL, isWatching, users))
}

// AddWatcher implementa POST /rest/api/3/issue/{issueIdOrKey}/watchers: il
// body opzionale è una raw accountId string; in sua assenza si usa il caller.
func (h *IssueHandler) AddWatcher(w http.ResponseWriter, r *http.Request) {
	iss, err := h.resolveIssue(r.PathValue("issueIdOrKey"))
	if err != nil || iss == nil {
		v3.WriteError(w, http.StatusNotFound, []string{"Issue does not exist or you do not have permission to see it."}, nil)
		return
	}
	userID := middleware.UserIDFromContext(r.Context())
	var raw string
	if json.NewDecoder(r.Body).Decode(&raw) == nil && raw != "" {
		userID = raw
	}
	if err := h.svc.Watch(iss.ID, userID); err != nil {
		v3.WriteError(w, http.StatusInternalServerError, []string{"Failed to add watcher."}, nil)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// RemoveWatcher implementa DELETE /rest/api/3/issue/{issueIdOrKey}/watchers:
// Jira passa l'utente target come query string ?accountId=; in sua assenza si
// usa il caller.
func (h *IssueHandler) RemoveWatcher(w http.ResponseWriter, r *http.Request) {
	iss, err := h.resolveIssue(r.PathValue("issueIdOrKey"))
	if err != nil || iss == nil {
		v3.WriteError(w, http.StatusNotFound, []string{"Issue does not exist or you do not have permission to see it."}, nil)
		return
	}
	userID := middleware.UserIDFromContext(r.Context())
	if q := r.URL.Query().Get("accountId"); q != "" {
		userID = q
	}
	if err := h.svc.Unwatch(iss.ID, userID); err != nil {
		v3.WriteError(w, http.StatusInternalServerError, []string{"Failed to remove watcher."}, nil)
		return
	}
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
