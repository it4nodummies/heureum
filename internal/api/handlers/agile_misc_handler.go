package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"

	v3 "github.com/open-jira/open-jira/internal/api/v3"
	"github.com/open-jira/open-jira/internal/domain/issue"
	"github.com/open-jira/open-jira/internal/domain/sprint"
)

type AgileMiscHandler struct {
	issueSvc  *issue.Service
	sprintSvc *sprint.Service
	issueH    *IssueHandler
	baseURL   string
}

func NewAgileMiscHandler(issueSvc *issue.Service, sprintSvc *sprint.Service, issueH *IssueHandler, baseURL string) *AgileMiscHandler {
	return &AgileMiscHandler{issueSvc: issueSvc, sprintSvc: sprintSvc, issueH: issueH, baseURL: baseURL}
}

func (h *AgileMiscHandler) resolveIssueID(idOrKey string) (string, bool) {
	if n, err := strconv.ParseInt(idOrKey, 10, 64); err == nil {
		if iss, err := h.issueSvc.GetBySeqID(n); err == nil {
			return iss.ID, true
		}
		return "", false
	}
	iss, err := h.issueSvc.GetByKey(idOrKey)
	if err != nil {
		return "", false
	}
	return iss.ID, true
}

// Rank: PUT /rest/agile/1.0/issue/rank.
func (h *AgileMiscHandler) Rank(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Issues          []string `json:"issues"`
		RankBeforeIssue string   `json:"rankBeforeIssue"`
		RankAfterIssue  string   `json:"rankAfterIssue"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		v3.WriteError(w, http.StatusBadRequest, []string{"invalid request body"}, nil)
		return
	}
	if len(req.Issues) == 0 {
		v3.WriteError(w, http.StatusBadRequest, []string{"issues is required"}, nil)
		return
	}
	ids := make([]string, 0, len(req.Issues))
	for _, k := range req.Issues {
		if id, ok := h.resolveIssueID(k); ok {
			ids = append(ids, id)
		}
	}
	var before, after *string
	if req.RankBeforeIssue != "" {
		if id, ok := h.resolveIssueID(req.RankBeforeIssue); ok {
			before = &id
		}
	}
	if req.RankAfterIssue != "" {
		if id, ok := h.resolveIssueID(req.RankAfterIssue); ok {
			after = &id
		}
	}
	if err := h.issueSvc.Rank(ids, before, after); err != nil {
		v3.WriteError(w, http.StatusInternalServerError, []string{"failed to rank issues"}, nil)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// MoveToBacklog: POST /rest/agile/1.0/backlog/issue — rimuove le issue dallo sprint.
func (h *AgileMiscHandler) MoveToBacklog(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Issues []string `json:"issues"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		v3.WriteError(w, http.StatusBadRequest, []string{"invalid request body"}, nil)
		return
	}
	for _, k := range req.Issues {
		if id, ok := h.resolveIssueID(k); ok {
			if err := h.sprintSvc.RemoveIssue(id); err != nil {
				v3.WriteError(w, http.StatusInternalServerError, []string{"failed to move to backlog"}, nil)
				return
			}
		}
	}
	w.WriteHeader(http.StatusNoContent)
}

// GetIssue: GET /rest/agile/1.0/issue/{issueIdOrKey} — IssueBean con extra sprint.
func (h *AgileMiscHandler) GetIssue(w http.ResponseWriter, r *http.Request) {
	iss := h.resolveIssue(r.PathValue("issueIdOrKey"))
	if iss == nil {
		v3.WriteError(w, http.StatusNotFound, []string{"issue not found"}, nil)
		return
	}
	bean := v3.JiraIssue(h.issueH.buildIssueInput(iss))
	m, err := v3.ProjectIssue(bean, v3.ParseFieldsFromList(nil))
	if err != nil {
		v3.WriteError(w, http.StatusInternalServerError, []string{"render error"}, nil)
		return
	}
	// arricchisce fields.sprint se la issue è in uno sprint
	if iss.SprintID != nil {
		if sp, err := h.sprintSvc.GetByID(*iss.SprintID); err == nil {
			if fields, ok := m["fields"].(map[string]any); ok {
				fields["sprint"] = sprintToV3(sp, h.baseURL)
			}
		}
	}
	v3.WriteJSON(w, http.StatusOK, m)
}

// GetEpic: GET /rest/agile/1.0/epic/{epicIdOrKey} — shape epic minimale.
func (h *AgileMiscHandler) GetEpic(w http.ResponseWriter, r *http.Request) {
	iss := h.resolveIssue(r.PathValue("epicIdOrKey"))
	if iss == nil {
		v3.WriteError(w, http.StatusNotFound, []string{"epic not found"}, nil)
		return
	}
	v3.WriteJSON(w, http.StatusOK, map[string]any{
		"id":      iss.SeqID,
		"key":     iss.Key,
		"self":    h.baseURL + "/rest/agile/1.0/epic/" + iss.Key,
		"name":    iss.Title,
		"summary": iss.Title,
		"done":    false,
	})
}

func (h *AgileMiscHandler) resolveIssue(idOrKey string) *issue.Issue {
	if n, err := strconv.ParseInt(idOrKey, 10, 64); err == nil {
		if iss, err := h.issueSvc.GetBySeqID(n); err == nil {
			return iss
		}
		return nil
	}
	iss, err := h.issueSvc.GetByKey(idOrKey)
	if err != nil {
		return nil
	}
	return iss
}
