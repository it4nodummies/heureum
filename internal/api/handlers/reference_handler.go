// Package handlers: ReferenceHandler espone i dati di riferimento Jira v3
// (priority, issuetype, status, resolution) nel formato ufficiale
// (schemi Priority, IssueTypeDetails, StatusDetails, Resolution).
package handlers

import (
	"net/http"
	"strconv"

	"github.com/it4nodummies/heureum/internal/api/authz"
	"github.com/it4nodummies/heureum/internal/api/middleware"
	v3 "github.com/it4nodummies/heureum/internal/api/v3"
	"github.com/it4nodummies/heureum/internal/domain/issue"
	"github.com/it4nodummies/heureum/internal/domain/project"
	"github.com/it4nodummies/heureum/internal/domain/workflow"
	"gorm.io/gorm"
)

type ReferenceHandler struct {
	db         *gorm.DB
	baseURL    string
	chk        *authz.Checker
	projectSvc *project.Service
}

// NewReferenceHandler costruisce lo handler. chk/projectSvc servono a
// scopare i cataloghi /label e /field ai soli progetti browsable dal
// chiamante (Task 8, Round 12): un admin globale vede tutto, invariato.
func NewReferenceHandler(db *gorm.DB, baseURL string, chk *authz.Checker, projectSvc *project.Service) *ReferenceHandler {
	return &ReferenceHandler{db: db, baseURL: baseURL, chk: chk, projectSvc: projectSvc}
}

// browsableProjectsScope restituisce la subquery dei progetti di cui uid è
// membro, o nil (nessuno scoping) se uid è admin globale o se chk/projectSvc
// non sono stati iniettati (compat test) — stesso pattern di
// AgileBoardHandler.boardScope / SearchHandler.searchScope.
func (h *ReferenceHandler) browsableProjectsScope(uid string) *gorm.DB {
	if h.chk == nil || h.projectSvc == nil || h.chk.IsGlobalAdmin(uid) {
		return nil
	}
	return h.projectSvc.MembershipSubquery(uid)
}

// Priorities → GET /rest/api/3/priority: le 5 priorità standard di Jira.
func (h *ReferenceHandler) Priorities(w http.ResponseWriter, r *http.Request) {
	v3.WriteJSON(w, http.StatusOK, v3.StandardPriorities(h.baseURL))
}

// defaultIssueTypes: usati quando la tabella issue_types non ha ancora righe
// (es. nessun progetto creato) così l'array resta comunque non vuoto.
var defaultIssueTypes = []struct {
	id, name, icon string
	subtask        bool
}{
	{"1", "Bug", "bug", false},
	{"2", "Task", "task", false},
	{"3", "Story", "story", false},
	{"4", "Epic", "epic", false},
	{"5", "Subtask", "subtask", true},
}

// IssueTypes → GET /rest/api/3/issuetype.
func (h *ReferenceHandler) IssueTypes(w http.ResponseWriter, r *http.Request) {
	var rows []issue.IssueType
	if err := h.db.Order("name ASC").Find(&rows).Error; err != nil {
		v3.WriteError(w, http.StatusInternalServerError, []string{"failed to list issue types"}, nil)
		return
	}

	out := make([]v3.IssueTypeRef, 0, len(rows))
	seen := map[string]bool{}
	for _, it := range rows {
		if seen[it.Name] {
			continue
		}
		seen[it.Name] = true
		out = append(out, v3.JiraIssueType(it.ID, it.Name, issueTypeIconURL(it.Icon, h.baseURL), it.IsSubtask, h.baseURL))
	}
	if len(out) == 0 {
		for _, d := range defaultIssueTypes {
			out = append(out, v3.JiraIssueType(d.id, d.name, issueTypeIconURL(d.icon, h.baseURL), d.subtask, h.baseURL))
		}
	}
	v3.WriteJSON(w, http.StatusOK, out)
}

func issueTypeIconURL(icon, baseURL string) string {
	if icon == "" {
		icon = "task"
	}
	return baseURL + "/static/issuetype-" + icon + ".svg"
}

// defaultStatuses: fallback quando workflow_statuses è vuota.
var defaultStatuses = []struct {
	id, name, category string
}{
	{"1", "To Do", string(workflow.CategoryTodo)},
	{"2", "In Progress", string(workflow.CategoryInProgress)},
	{"3", "Done", string(workflow.CategoryDone)},
}

// Statuses → GET /rest/api/3/status.
func (h *ReferenceHandler) Statuses(w http.ResponseWriter, r *http.Request) {
	var rows []workflow.WorkflowStatus
	if err := h.db.Order("name ASC").Find(&rows).Error; err != nil {
		v3.WriteError(w, http.StatusInternalServerError, []string{"failed to list statuses"}, nil)
		return
	}

	out := make([]v3.StatusRef, 0, len(rows))
	seen := map[string]bool{}
	for _, st := range rows {
		if seen[st.Name] {
			continue
		}
		seen[st.Name] = true
		out = append(out, v3.JiraStatus(st.ID, st.Name, string(st.Category), h.baseURL))
	}
	if len(out) == 0 {
		for _, d := range defaultStatuses {
			out = append(out, v3.JiraStatus(d.id, d.name, d.category, h.baseURL))
		}
	}
	v3.WriteJSON(w, http.StatusOK, out)
}

// allCategories restituisce le 3 status-category emesse dal sistema.
func (h *ReferenceHandler) allCategories() []v3.StatusCategoryRef {
	return []v3.StatusCategoryRef{
		v3.CategoryFor("todo", h.baseURL),
		v3.CategoryFor("inprogress", h.baseURL),
		v3.CategoryFor("done", h.baseURL),
	}
}

// StatusCategories → GET /rest/api/3/statuscategory.
func (h *ReferenceHandler) StatusCategories(w http.ResponseWriter, r *http.Request) {
	v3.WriteJSON(w, http.StatusOK, h.allCategories())
}

// StatusCategoryByID → GET /rest/api/3/statuscategory/{idOrKey} (per id numerico o key).
func (h *ReferenceHandler) StatusCategoryByID(w http.ResponseWriter, r *http.Request) {
	idOrKey := r.PathValue("idOrKey")
	for _, c := range h.allCategories() {
		if idOrKey == c.Key || idOrKey == strconv.Itoa(c.ID) {
			v3.WriteJSON(w, http.StatusOK, c)
			return
		}
	}
	v3.WriteError(w, http.StatusNotFound, []string{"status category not found"}, nil)
}

// resolutionRow mappa la tabella "resolutions" (id, name, description): non
// esiste ancora un modello di dominio dedicato, quindi leggiamo direttamente
// con GORM fissando il nome tabella.
type resolutionRow struct {
	ID          string `gorm:"column:id"`
	Name        string `gorm:"column:name"`
	Description string `gorm:"column:description"`
}

func (resolutionRow) TableName() string { return "resolutions" }

// defaultResolutions: fallback quando la tabella resolutions è vuota.
var defaultResolutions = []struct {
	id, name, description string
}{
	{"1", "Fixed", "A fix for this issue is checked into the tree and tested."},
	{"2", "Won't Fix", "The problem described is an issue which will never be fixed."},
	{"3", "Duplicate", "The problem is a duplicate of an existing issue."},
	{"4", "Cannot Reproduce", "All attempts at reproducing this issue failed, or not enough information was available."},
	{"5", "Done", "Work has been completed."},
}

// CreateMeta → GET /rest/api/3/issue/createmeta: forma minima conforme allo
// schema IssueCreateMetadata ({expand, projects}, additionalProperties:false).
// Non popoliamo ancora projects/issuetypes/fields per-progetto: un array vuoto
// resta valido secondo lo schema (nessuna proprietà è required).
func (h *ReferenceHandler) CreateMeta(w http.ResponseWriter, r *http.Request) {
	v3.WriteJSON(w, http.StatusOK, map[string]any{"projects": []any{}})
}

// EditMeta → GET /rest/api/3/issue/{issueIdOrKey}/editmeta: forma minima
// conforme allo schema IssueUpdateMetadata ({fields}, additionalProperties
// permissivo). fields vuoto resta valido: nessuna proprietà è required.
func (h *ReferenceHandler) EditMeta(w http.ResponseWriter, r *http.Request) {
	v3.WriteJSON(w, http.StatusOK, map[string]any{"fields": map[string]any{}})
}

// Fields → GET /rest/api/3/field: campi standard di sistema più i custom
// field del progetto (tabella custom_fields), nel formato FieldDetails
// (additionalProperties:false — solo le chiavi consentite dallo schema).
func (h *ReferenceHandler) Fields(w http.ResponseWriter, r *http.Request) {
	type fieldOut struct {
		ID         string `json:"id"`
		Key        string `json:"key"`
		Name       string `json:"name"`
		Custom     bool   `json:"custom"`
		Orderable  bool   `json:"orderable"`
		Navigable  bool   `json:"navigable"`
		Searchable bool   `json:"searchable"`
	}
	out := []fieldOut{
		{ID: "summary", Key: "summary", Name: "Summary", Navigable: true, Orderable: true, Searchable: true},
		{ID: "issuetype", Key: "issuetype", Name: "Issue Type", Navigable: true, Orderable: true, Searchable: true},
		{ID: "status", Key: "status", Name: "Status", Navigable: true, Searchable: true},
		{ID: "priority", Key: "priority", Name: "Priority", Navigable: true, Orderable: true, Searchable: true},
		{ID: "assignee", Key: "assignee", Name: "Assignee", Navigable: true, Orderable: true, Searchable: true},
		{ID: "labels", Key: "labels", Name: "Labels", Navigable: true, Orderable: true, Searchable: true},
		{ID: "description", Key: "description", Name: "Description", Navigable: true, Searchable: true},
	}
	uid := middleware.UserIDFromContext(r.Context())
	type cf struct{ ID, Name string }
	var customs []cf
	q := h.db.Table("custom_fields").Select("id, name")
	if scope := h.browsableProjectsScope(uid); scope != nil {
		q = q.Where("project_id IN (?)", scope)
	}
	q.Scan(&customs)
	for _, c := range customs {
		out = append(out, fieldOut{ID: "customfield_" + c.ID, Key: "customfield_" + c.ID, Name: c.Name, Custom: true, Navigable: true, Orderable: true, Searchable: true})
	}
	v3.WriteJSON(w, http.StatusOK, out)
}

// Labels → GET /rest/api/3/label: PageBeanString con i nomi distinti di label
// esistenti (tabella labels, colonna name, con project_id). Scopato ai
// progetti browsable dal chiamante (non admin globale) per non far trapelare
// nomi di label di progetti a cui non ha accesso.
func (h *ReferenceHandler) Labels(w http.ResponseWriter, r *http.Request) {
	uid := middleware.UserIDFromContext(r.Context())
	var names []string
	q := h.db.Table("labels").Distinct("name")
	if scope := h.browsableProjectsScope(uid); scope != nil {
		q = q.Where("project_id IN (?)", scope)
	}
	q.Pluck("name", &names)
	if names == nil {
		names = []string{}
	}
	v3.WritePage(w, http.StatusOK, v3.Page[string]{StartAt: 0, MaxResults: 1000, Total: len(names), Values: names})
}

// Resolutions → GET /rest/api/3/resolution.
func (h *ReferenceHandler) Resolutions(w http.ResponseWriter, r *http.Request) {
	var rows []resolutionRow
	if err := h.db.Order("name ASC").Find(&rows).Error; err != nil {
		v3.WriteError(w, http.StatusInternalServerError, []string{"failed to list resolutions"}, nil)
		return
	}

	out := make([]v3.ResolutionRef, 0, len(rows))
	for _, res := range rows {
		out = append(out, v3.JiraResolution(res.ID, res.Name, res.Description, h.baseURL))
	}
	if len(out) == 0 {
		for _, d := range defaultResolutions {
			out = append(out, v3.JiraResolution(d.id, d.name, d.description, h.baseURL))
		}
	}
	v3.WriteJSON(w, http.StatusOK, out)
}
