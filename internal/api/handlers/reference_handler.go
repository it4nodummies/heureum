// Package handlers: ReferenceHandler espone i dati di riferimento Jira v3
// (priority, issuetype, status, resolution) nel formato ufficiale
// (schemi Priority, IssueTypeDetails, StatusDetails, Resolution).
package handlers

import (
	"net/http"

	v3 "github.com/open-jira/open-jira/internal/api/v3"
	"github.com/open-jira/open-jira/internal/domain/issue"
	"github.com/open-jira/open-jira/internal/domain/workflow"
	"gorm.io/gorm"
)

type ReferenceHandler struct {
	db      *gorm.DB
	baseURL string
}

func NewReferenceHandler(db *gorm.DB, baseURL string) *ReferenceHandler {
	return &ReferenceHandler{db: db, baseURL: baseURL}
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
