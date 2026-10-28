package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/it4nodummies/heureum/internal/api/authz"
	"github.com/it4nodummies/heureum/internal/api/middleware"
	v3 "github.com/it4nodummies/heureum/internal/api/v3"
	"github.com/it4nodummies/heureum/internal/domain/issue"
	"github.com/it4nodummies/heureum/internal/domain/permission"
	"github.com/it4nodummies/heureum/internal/domain/sprint"
)

type AgileMiscHandler struct {
	issueSvc  *issue.Service
	sprintSvc *sprint.Service
	issueH    *IssueHandler
	chk       *authz.Checker
	baseURL   string
}

func NewAgileMiscHandler(issueSvc *issue.Service, sprintSvc *sprint.Service, issueH *IssueHandler, chk *authz.Checker, baseURL string) *AgileMiscHandler {
	return &AgileMiscHandler{issueSvc: issueSvc, sprintSvc: sprintSvc, issueH: issueH, chk: chk, baseURL: baseURL}
}

// authorizeBodyIssues resolves every issue named in a request body and checks
// that uid holds permKey on EACH issue's project. These routes have no path
// context, so the body issues are the only thing there is to authorize: an
// unresolvable key/id and an issue the caller cannot browse look the same
// (404, Jira's wording) so a guessed key reveals nothing; an issue the caller
// can browse but not manage is a plain 403. It returns the internal ids in
// body order, or the status/message to write. Callers must not write anything
// before this returns ok: the whole batch is validated first (no partial
// writes).
func (h *AgileMiscHandler) authorizeBodyIssues(uid string, keys []string, permKey string) (ids []string, status int, msg string) {
	ids = make([]string, 0, len(keys))
	for _, k := range keys {
		iss := h.resolveIssue(k)
		if iss == nil || h.chk.RequireProject(uid, iss.ProjectID, permission.BrowseProjects) != nil {
			return nil, http.StatusNotFound, "Issue does not exist or you do not have permission to see it."
		}
		if err := h.chk.RequireProject(uid, iss.ProjectID, permKey); err != nil {
			return nil, http.StatusForbidden, "you do not have permission to perform this action"
		}
		ids = append(ids, iss.ID)
	}
	return ids, 0, ""
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
	// Every issue in the body — the ranked ones AND the rankBefore/rankAfter
	// anchors, whose positions are read — needs ManageSprints on its project.
	uid := middleware.UserIDFromContext(r.Context())
	ids, status, msg := h.authorizeBodyIssues(uid, req.Issues, permission.ManageSprints)
	if status != 0 {
		v3.WriteError(w, status, []string{msg}, nil)
		return
	}
	var before, after *string
	if req.RankBeforeIssue != "" {
		anchor, status, msg := h.authorizeBodyIssues(uid, []string{req.RankBeforeIssue}, permission.ManageSprints)
		if status != 0 {
			v3.WriteError(w, status, []string{msg}, nil)
			return
		}
		before = &anchor[0]
	}
	if req.RankAfterIssue != "" {
		anchor, status, msg := h.authorizeBodyIssues(uid, []string{req.RankAfterIssue}, permission.ManageSprints)
		if status != 0 {
			v3.WriteError(w, status, []string{msg}, nil)
			return
		}
		after = &anchor[0]
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
	// Every issue in the body needs ManageSprints on its own project; the
	// batch is validated as a whole before the first write (no partial move).
	uid := middleware.UserIDFromContext(r.Context())
	ids, status, msg := h.authorizeBodyIssues(uid, req.Issues, permission.ManageSprints)
	if status != 0 {
		v3.WriteError(w, status, []string{msg}, nil)
		return
	}
	for _, id := range ids {
		if err := h.sprintSvc.RemoveIssue(id); err != nil {
			v3.WriteError(w, http.StatusInternalServerError, []string{"failed to move to backlog"}, nil)
			return
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
		if sp, err := h.sprintSvc.GetByID(iss.ProjectID, *iss.SprintID); err == nil {
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
