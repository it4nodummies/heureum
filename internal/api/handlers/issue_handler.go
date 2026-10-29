package handlers

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"

	"gorm.io/gorm"

	"github.com/it4nodummies/heureum/internal/api/authz"
	"github.com/it4nodummies/heureum/internal/api/middleware"
	v3 "github.com/it4nodummies/heureum/internal/api/v3"
	"github.com/it4nodummies/heureum/internal/domain/issue"
	"github.com/it4nodummies/heureum/internal/domain/permission"
	"github.com/it4nodummies/heureum/internal/domain/project"
	"github.com/it4nodummies/heureum/internal/domain/user"
	"github.com/it4nodummies/heureum/internal/domain/version"
	"github.com/it4nodummies/heureum/internal/domain/workflow"
)

type IssueHandler struct {
	svc        *issue.Service
	projectSvc *project.Service
	wfSvc      *workflow.Service
	chk        *authz.Checker
	versionSvc *version.Service
	baseURL    string
}

func NewIssueHandler(svc *issue.Service, projectSvc *project.Service, wfSvc *workflow.Service, chk *authz.Checker, versionSvc *version.Service, baseURL string) *IssueHandler {
	return &IssueHandler{svc: svc, projectSvc: projectSvc, wfSvc: wfSvc, chk: chk, versionSvc: versionSvc, baseURL: baseURL}
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

// isEpic verifica se la issue è un epic: tipo di livello gerarchico 1
// (Round 28; prima si confrontava il NOME del tipo con "Epic").
func (h *IssueHandler) isEpic(iss *issue.Issue) bool {
	if iss.TypeID == nil {
		return false
	}
	var it issue.IssueType
	if h.svc.DB().Select("hierarchy_level").First(&it, "id = ?", *iss.TypeID).Error != nil {
		return false
	}
	return it.HierarchyLevel == 1
}

// parentRefs è la cache per-render dei fields.parent già costruiti, chiave
// l'id interno del parent. renderIssueList/Subtasks la riempiono in blocco
// con loadParentRefs (3 query IN per l'intera lista) ed evitano così una
// query per riga; buildIssueInput senza cache carica il solo parent della
// issue.
type parentRefs map[string]*v3.ParentRef

// loadParentRefs costruisce i ParentRef (id/key/self + fields summary,
// status, issuetype, priority) degli issue con gli id dati, in tre query:
// parents, i loro status, i loro tipi. Parent archiviati restano esposti:
// il link esiste ancora e la UI deve poterlo mostrare/azzerare.
func (h *IssueHandler) loadParentRefs(ids []string) parentRefs {
	out := parentRefs{}
	if len(ids) == 0 {
		return out
	}
	db := h.svc.DB()
	var parents []issue.Issue
	if err := db.Where("id IN ?", ids).Find(&parents).Error; err != nil {
		log.Printf("issue: load parents failed: %v", err)
		return out
	}
	statusIDs, typeIDs := make([]string, 0, len(parents)), make([]string, 0, len(parents))
	for _, p := range parents {
		if p.StatusID != nil {
			statusIDs = append(statusIDs, *p.StatusID)
		}
		if p.TypeID != nil {
			typeIDs = append(typeIDs, *p.TypeID)
		}
	}
	statuses := map[string]workflow.WorkflowStatus{}
	if len(statusIDs) > 0 {
		var rows []workflow.WorkflowStatus
		if err := db.Where("id IN ?", statusIDs).Find(&rows).Error; err == nil {
			for _, st := range rows {
				statuses[st.ID] = st
			}
		}
	}
	types := map[string]issue.IssueType{}
	if len(typeIDs) > 0 {
		var rows []issue.IssueType
		if err := db.Where("id IN ?", typeIDs).Find(&rows).Error; err == nil {
			for _, it := range rows {
				types[it.ID] = it
			}
		}
	}
	for i := range parents {
		p := parents[i]
		pf := &v3.ParentFields{
			Summary:  p.Title,
			Priority: issuePriorityRef(string(p.Priority), h.baseURL),
		}
		if p.StatusID != nil {
			if st, ok := statuses[*p.StatusID]; ok {
				s := v3.JiraStatus(st.ID, st.Name, string(st.Category), h.baseURL)
				pf.Status = &s
			}
		}
		if p.TypeID != nil {
			if it, ok := types[*p.TypeID]; ok {
				ref := v3.JiraIssueTypeFrom(it, h.baseURL)
				pf.IssueType = &ref
			}
		}
		out[p.ID] = &v3.ParentRef{
			ID:     itoaInt64(p.SeqID),
			Key:    p.Key,
			Self:   h.baseURL + "/rest/api/3/issue/" + itoaInt64(p.SeqID),
			Fields: pf,
		}
	}
	return out
}

// issuePriorityRef è il PriorityRef v3 di una priority interna (pointer).
func issuePriorityRef(priority, baseURL string) *v3.PriorityRef {
	ref := v3.PriorityForEnum(priority, baseURL)
	return &ref
}

// parentIDsOf raccoglie gli id parent distinti (non nil) delle issue.
func parentIDsOf(issues []issue.Issue) []string {
	seen := map[string]bool{}
	ids := make([]string, 0)
	for i := range issues {
		if pid := issues[i].ParentID; pid != nil && !seen[*pid] {
			seen[*pid] = true
			ids = append(ids, *pid)
		}
	}
	return ids
}

// buildIssueInput arricchisce un'issue di dominio con le entità collegate
// (assignee, reporter, tipo, stato, progetto, parent, label) necessarie a
// costruire un v3.IssueBean completo tramite v3.JiraIssue.
func (h *IssueHandler) buildIssueInput(iss *issue.Issue) v3.IssueInput {
	return h.buildIssueInputWith(iss, nil)
}

// buildIssueInputWith è buildIssueInput con una cache dei parent già
// caricati (nil = carica il parent di questa sola issue).
func (h *IssueHandler) buildIssueInputWith(iss *issue.Issue, parents parentRefs) v3.IssueInput {
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
		if parents == nil {
			parents = h.loadParentRefs([]string{*iss.ParentID})
		}
		in.Parent = parents[*iss.ParentID]
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
	if h.versionSvc != nil {
		fvs, _ := h.versionSvc.GetFixVersions(iss.ID)
		refs := make([]v3.VersionRef, 0, len(fvs))
		for _, fv := range fvs {
			refs = append(refs, v3.VersionRef{
				Self: h.baseURL + "/rest/api/3/version/" + fv.ID,
				ID:   fv.ID,
				Name: fv.Name,
			})
		}
		in.FixVersions = refs
	}
	return in
}

// priorityEnumForID mappa gli ID di priorità Jira standard (1..5) all'enum
// interno usato da issue.Priority.
func priorityEnumForID(id string) string {
	return map[string]string{"1": "highest", "2": "high", "3": "medium", "4": "low", "5": "lowest"}[id]
}

// parentSpec è fields.parent come lo manda un client Jira: {"key":"PRJ-1"}
// oppure {"id":"10001"} (l'id numerico, accettato anche come numero JSON
// grazie a json.Number). Vince la key se entrambi sono presenti.
type parentSpec struct {
	Key string      `json:"key"`
	ID  json.Number `json:"id"`
}

// resolveParentSpec mappa un parentSpec all'id interno del parent: nil spec
// → nil (nessun parent); key/id sconosciuti o archiviati → ErrParentNotFound;
// spec senza key né id → ErrParentNotFound. Le regole gerarchiche (livello,
// progetto, cicli) restano al dominio (createOnce/SetParent).
func (h *IssueHandler) resolveParentSpec(spec *parentSpec) (*string, error) {
	if spec == nil {
		return nil, nil
	}
	idOrKey := spec.Key
	if idOrKey == "" {
		idOrKey = spec.ID.String()
	}
	if idOrKey == "" {
		return nil, issue.ErrParentNotFound
	}
	parent, err := h.resolveIssue(idOrKey)
	if err != nil || parent == nil {
		return nil, issue.ErrParentNotFound
	}
	return &parent.ID, nil
}

// parentErrorMessage è il messaggio errors.parent per i sentinel del
// dominio: "Parent issue not found." oppure la ragione che ErrParentInvalid
// porta con sé ("Parent must be an Epic.").
func parentErrorMessage(err error) string {
	if errors.Is(err, issue.ErrParentInvalid) {
		reason := strings.TrimPrefix(err.Error(), issue.ErrParentInvalid.Error()+": ")
		if reason == "" || reason == err.Error() {
			return "The parent is not valid for this issue."
		}
		return strings.ToUpper(reason[:1]) + reason[1:] + "."
	}
	return "Parent issue not found."
}

func (h *IssueHandler) ExportCSV(w http.ResponseWriter, r *http.Request) {
	p, err := h.projectSvc.GetByKey(r.PathValue("key"))
	if err != nil {
		http.Error(w, `{"error":"project not found"}`, http.StatusNotFound)
		return
	}
	rows, err := h.svc.ExportRows(p.ID)
	if err != nil {
		http.Error(w, `{"error":"export failed"}`, http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/csv")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s-issues.csv", p.Key))
	wr := csv.NewWriter(w)
	wr.Write([]string{"Key", "Title", "Priority", "Status", "Type", "Assignee", "Story Points", "Created", "Updated"})
	for _, row := range rows {
		wr.Write([]string{
			row.Key,
			row.Title,
			row.Priority,
			row.Status,
			row.Type,
			row.Assignee,
			fmt.Sprintf("%d", row.StoryPoints),
			row.Created.Format("2006-01-02"),
			row.Updated.Format("2006-01-02"),
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
			Parent      *parentSpec `json:"parent"`
			Labels      []string    `json:"labels"`
			FixVersions []struct {
				ID string `json:"id"`
			} `json:"fixVersions"`
			Assignee *struct {
				AccountID string `json:"accountId"`
			} `json:"assignee"`
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
	// An unknown parent is a client error (400 errors.parent), never a
	// silent drop; the hierarchy rules themselves (level, project, cycle)
	// are enforced by the domain on create, after the type is resolved below.
	parentID, perr := h.resolveParentSpec(req.Fields.Parent)
	if perr != nil {
		v3.WriteError(w, http.StatusBadRequest, nil, map[string]string{"parent": parentErrorMessage(perr)})
		return
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

	// Create-time follow-ups (initial status, assignee, labels, fix versions)
	// go INTO the create transaction via CreateOptions: they used to be four
	// separate UPDATEs after the insert, each with its error discarded, so a
	// failure left a half-initialized issue (and one of them could nil-deref
	// the handler). Now either the whole issue exists or none of it does.
	opts := issue.CreateOptions{Labels: req.Fields.Labels}
	// Mantiene l'auto-assegnazione dello status "todo" dal workflow del progetto.
	if h.wfSvc != nil {
		if wf, wfErr := h.wfSvc.GetWorkflow(proj.ID); wfErr == nil {
			for _, status := range wf.Statuses {
				if status.Category == workflow.CategoryTodo {
					id := status.ID
					opts.StatusID = &id
					break
				}
			}
		}
	}
	if req.Fields.Assignee != nil && req.Fields.Assignee.AccountID != "" {
		accountID := req.Fields.Assignee.AccountID
		opts.AssigneeID = &accountID
	}
	if len(req.Fields.FixVersions) > 0 && h.versionSvc != nil {
		ids := make([]string, 0, len(req.Fields.FixVersions))
		for _, fv := range req.Fields.FixVersions {
			if fv.ID != "" {
				ids = append(ids, fv.ID)
			}
		}
		opts.Extra = func(tx *gorm.DB, iss *issue.Issue) error {
			return h.versionSvc.SetFixVersionsIn(tx, iss.ID, ids)
		}
	}

	iss, err := h.svc.CreateWithOptions(proj.Key, proj.ID, req.Fields.Summary, descJSON, priority, parentID, typeID, opts)
	switch {
	case errors.Is(err, issue.ErrIssueTypeNotFound):
		v3.WriteError(w, http.StatusBadRequest, nil, map[string]string{"issuetype": "The issue type selected is invalid."})
		return
	case errors.Is(err, issue.ErrAssigneeNotFound):
		v3.WriteError(w, http.StatusBadRequest, nil, map[string]string{"assignee": "User '" + req.Fields.Assignee.AccountID + "' does not exist."})
		return
	case errors.Is(err, issue.ErrProjectNotFound):
		v3.WriteError(w, http.StatusBadRequest, nil, map[string]string{"project": "The project does not exist."})
		return
	case errors.Is(err, issue.ErrParentNotFound), errors.Is(err, issue.ErrParentInvalid):
		v3.WriteError(w, http.StatusBadRequest, nil, map[string]string{"parent": parentErrorMessage(err)})
		return
	case err != nil:
		// Anything else is internal (DB failure, rolled back): log the
		// detail, keep the client message generic.
		log.Printf("issue: create in %q failed: %v", proj.Key, err)
		v3.WriteError(w, http.StatusInternalServerError, []string{"Failed to create issue."}, nil)
		return
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
			FixVersions *[]struct {
				ID string `json:"id"`
			} `json:"fixVersions"`
			Assignee *struct {
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
			// parent is kept raw to tell the three cases apart: absent
			// (len 0 → unchanged), explicit null ("null" → clear), or an
			// object {key}|{id} (→ move). A *struct would collapse null and
			// absent into nil.
			Parent json.RawMessage `json:"parent"`
		} `json:"fields"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		v3.WriteError(w, http.StatusBadRequest, []string{"Invalid request body."}, nil)
		return
	}
	// The parent goes first: it is the only field with validation that can
	// reject the request (400), so an invalid parent leaves the issue
	// untouched instead of half-updated. Resolution of key/id happens
	// before any write for the same reason.
	if len(req.Fields.Parent) > 0 {
		var spec *parentSpec
		if string(bytes.TrimSpace(req.Fields.Parent)) != "null" {
			spec = &parentSpec{}
			if err := json.Unmarshal(req.Fields.Parent, spec); err != nil {
				v3.WriteError(w, http.StatusBadRequest, nil, map[string]string{"parent": "Parent must be an object with key or id, or null."})
				return
			}
		}
		parentID, perr := h.resolveParentSpec(spec)
		if perr == nil {
			perr = h.svc.SetParent(iss.Key, parentID)
		}
		switch {
		case errors.Is(perr, issue.ErrParentNotFound), errors.Is(perr, issue.ErrParentInvalid):
			v3.WriteError(w, http.StatusBadRequest, nil, map[string]string{"parent": parentErrorMessage(perr)})
			return
		case perr != nil:
			log.Printf("issue: set parent of %s failed: %v", iss.Key, perr)
			v3.WriteError(w, http.StatusInternalServerError, []string{"Failed to update issue."}, nil)
			return
		}
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
	// fixVersions è un pointer slice: nil = campo assente (invariato), []{} = azzera.
	if req.Fields.FixVersions != nil && h.versionSvc != nil {
		ids := make([]string, 0, len(*req.Fields.FixVersions))
		for _, fv := range *req.Fields.FixVersions {
			if fv.ID != "" {
				ids = append(ids, fv.ID)
			}
		}
		if err := h.versionSvc.SetFixVersions(iss.ID, ids); err != nil {
			v3.WriteError(w, http.StatusInternalServerError, []string{"Failed to update fix versions."}, nil)
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
// figli DIRETTI (issue.parent_id == issue.ID) come v3 issue completi, riusando
// la stessa costruzione (buildIssueInput + v3.JiraIssue) di Get. "Subtasks"
// è il nome Jira della rotta, ma la lista copre i figli di qualunque livello:
// su un epic restituisce le sue story/task/bug, su una story le sue subtask
// (Round 28, gerarchia stretta Epic → Story → Subtask).
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
	// Every child shares the same parent: one cached ParentRef for the list.
	parents := h.loadParentRefs(parentIDsOf(children))
	out := make([]v3.IssueBean, 0, len(children))
	for i := range children {
		out = append(out, v3.JiraIssue(h.buildIssueInputWith(&children[i], parents)))
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
