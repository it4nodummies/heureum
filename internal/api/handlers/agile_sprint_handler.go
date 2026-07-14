package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	v3 "github.com/it4nodummies/heureum/internal/api/v3"
	"github.com/it4nodummies/heureum/internal/domain/board"
	"github.com/it4nodummies/heureum/internal/domain/issue"
	"github.com/it4nodummies/heureum/internal/domain/sprint"
)

type AgileSprintHandler struct {
	sprintSvc *sprint.Service
	boardSvc  *board.Service
	issueSvc  *issue.Service
	issueH    *IssueHandler
	baseURL   string
}

func NewAgileSprintHandler(sprintSvc *sprint.Service, boardSvc *board.Service, issueSvc *issue.Service, issueH *IssueHandler, baseURL string) *AgileSprintHandler {
	return &AgileSprintHandler{sprintSvc: sprintSvc, boardSvc: boardSvc, issueSvc: issueSvc, issueH: issueH, baseURL: baseURL}
}

func (h *AgileSprintHandler) resolveSprint(r *http.Request) *sprint.Sprint {
	n, err := strconv.ParseInt(r.PathValue("sprintId"), 10, 64)
	if err != nil {
		return nil
	}
	sp, err := h.sprintSvc.GetBySeqID(n)
	if err != nil {
		return nil
	}
	return sp
}

// parseAgileTime interpreta le date del contratto (RFC3339 / ISO8601). Vuoto → nil.
func parseAgileTime(s string) *time.Time {
	if s == "" {
		return nil
	}
	for _, layout := range []string{time.RFC3339, "2006-01-02T15:04:05.000-07:00", "2006-01-02"} {
		if t, err := time.Parse(layout, s); err == nil {
			return &t
		}
	}
	return nil
}

func (h *AgileSprintHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name          string `json:"name"`
		Goal          string `json:"goal"`
		StartDate     string `json:"startDate"`
		EndDate       string `json:"endDate"`
		OriginBoardID int64  `json:"originBoardId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		v3.WriteError(w, http.StatusBadRequest, []string{"invalid request body"}, nil)
		return
	}
	if req.Name == "" || req.OriginBoardID == 0 {
		v3.WriteError(w, http.StatusBadRequest, []string{"name and originBoardId are required"}, nil)
		return
	}
	b, err := h.boardSvc.GetBySeqID(req.OriginBoardID)
	if err != nil {
		v3.WriteError(w, http.StatusBadRequest, []string{"originBoardId not found"}, nil)
		return
	}
	obid := req.OriginBoardID
	sp, err := h.sprintSvc.CreateFull(b.ProjectID, req.Name, req.Goal, &obid, parseAgileTime(req.StartDate), parseAgileTime(req.EndDate))
	if err != nil {
		v3.WriteError(w, http.StatusInternalServerError, []string{"failed to create sprint"}, nil)
		return
	}
	v3.WriteJSON(w, http.StatusCreated, sprintToV3(sp, h.baseURL))
}

func (h *AgileSprintHandler) Get(w http.ResponseWriter, r *http.Request) {
	sp := h.resolveSprint(r)
	if sp == nil {
		v3.WriteError(w, http.StatusNotFound, []string{"sprint not found"}, nil)
		return
	}
	v3.WriteJSON(w, http.StatusOK, sprintToV3(sp, h.baseURL))
}

// Update gestisce POST/PUT /sprint/{id}: aggiorna campi + transizioni di stato.
func (h *AgileSprintHandler) Update(w http.ResponseWriter, r *http.Request) {
	sp := h.resolveSprint(r)
	if sp == nil {
		v3.WriteError(w, http.StatusNotFound, []string{"sprint not found"}, nil)
		return
	}
	var req struct {
		Name      *string `json:"name"`
		Goal      *string `json:"goal"`
		State     *string `json:"state"`
		StartDate *string `json:"startDate"`
		EndDate   *string `json:"endDate"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		v3.WriteError(w, http.StatusBadRequest, []string{"invalid request body"}, nil)
		return
	}
	// Le transizioni active/closed passano per Start/Complete (side-effect su issue).
	if req.State != nil {
		switch *req.State {
		case "active":
			if _, err := h.sprintSvc.Start(sp.ID); err != nil {
				v3.WriteError(w, http.StatusBadRequest, []string{"cannot start sprint"}, nil)
				return
			}
		case "closed":
			if _, err := h.sprintSvc.Complete(sp.ID, true); err != nil {
				v3.WriteError(w, http.StatusBadRequest, []string{"cannot complete sprint"}, nil)
				return
			}
		}
	}
	var start, end *time.Time
	if req.StartDate != nil {
		start = parseAgileTime(*req.StartDate)
	}
	if req.EndDate != nil {
		end = parseAgileTime(*req.EndDate)
	}
	// name/goal/date (lo stato è già gestito sopra: non ripassarlo a UpdateFull)
	updated, err := h.sprintSvc.UpdateFull(sp.ID, req.Name, req.Goal, nil, start, end)
	if err != nil {
		v3.WriteError(w, http.StatusInternalServerError, []string{"failed to update sprint"}, nil)
		return
	}
	v3.WriteJSON(w, http.StatusOK, sprintToV3(updated, h.baseURL))
}

func (h *AgileSprintHandler) Delete(w http.ResponseWriter, r *http.Request) {
	sp := h.resolveSprint(r)
	if sp == nil {
		v3.WriteError(w, http.StatusNotFound, []string{"sprint not found"}, nil)
		return
	}
	if err := h.sprintSvc.DB().Where("id = ?", sp.ID).Delete(&sprint.Sprint{}).Error; err != nil {
		v3.WriteError(w, http.StatusInternalServerError, []string{"failed to delete sprint"}, nil)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// SprintIssues: GET issue dello sprint (SearchResults).
func (h *AgileSprintHandler) SprintIssues(w http.ResponseWriter, r *http.Request) {
	sp := h.resolveSprint(r)
	if sp == nil {
		v3.WriteError(w, http.StatusNotFound, []string{"sprint not found"}, nil)
		return
	}
	var issues []issue.Issue
	if err := h.issueSvc.DB().Where("sprint_id = ? AND is_archived = ?", sp.ID, false).Order("position ASC").Find(&issues).Error; err != nil {
		v3.WriteError(w, http.StatusInternalServerError, []string{"failed to list issues"}, nil)
		return
	}
	startAt, maxResults := v3.ParsePagination(r, 50, 100)
	items, err := renderIssueList(h.issueH, page(issues, startAt, maxResults), nil)
	if err != nil {
		v3.WriteError(w, http.StatusInternalServerError, []string{"render error"}, nil)
		return
	}
	v3.WriteJSON(w, http.StatusOK, v3.SearchResults{Issues: items, StartAt: startAt, MaxResults: maxResults, Total: len(issues)})
}

// MoveToSprint: POST /sprint/{id}/issue — sposta issue nello sprint.
func (h *AgileSprintHandler) MoveToSprint(w http.ResponseWriter, r *http.Request) {
	sp := h.resolveSprint(r)
	if sp == nil {
		v3.WriteError(w, http.StatusNotFound, []string{"sprint not found"}, nil)
		return
	}
	var req struct {
		Issues []string `json:"issues"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		v3.WriteError(w, http.StatusBadRequest, []string{"invalid request body"}, nil)
		return
	}
	for _, key := range req.Issues {
		iss := h.resolveIssue(key)
		if iss == nil {
			continue
		}
		if err := h.sprintSvc.AddIssue(sp.ID, iss.ID); err != nil {
			v3.WriteError(w, http.StatusInternalServerError, []string{"failed to move issue"}, nil)
			return
		}
	}
	w.WriteHeader(http.StatusNoContent)
}

// resolveIssue risolve una issue da id numerico (seq_id) o key.
func (h *AgileSprintHandler) resolveIssue(idOrKey string) *issue.Issue {
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
