package handlers

import (
	"encoding/json"
	"net/http"
	"sort"
	"strconv"

	"github.com/it4nodummies/heureum/internal/api/authz"
	"github.com/it4nodummies/heureum/internal/api/middleware"
	v3 "github.com/it4nodummies/heureum/internal/api/v3"
	"github.com/it4nodummies/heureum/internal/domain/board"
	"github.com/it4nodummies/heureum/internal/domain/issue"
	"github.com/it4nodummies/heureum/internal/domain/permission"
	"github.com/it4nodummies/heureum/internal/domain/project"
	"github.com/it4nodummies/heureum/internal/domain/sprint"
	"github.com/it4nodummies/heureum/internal/domain/workflow"
)

// AgileBoardHandler implementa gli endpoint /rest/agile/1.0/board/* (CRUD,
// configurazione, backlog/issue/sprint/epic — questi ultimi in Task 7).
type AgileBoardHandler struct {
	boardSvc    *board.Service
	projectSvc  *project.Service
	issueSvc    *issue.Service
	sprintSvc   *sprint.Service
	workflowSvc *workflow.Service
	issueH      *IssueHandler
	chk         *authz.Checker
	baseURL     string
}

func NewAgileBoardHandler(boardSvc *board.Service, projectSvc *project.Service, issueSvc *issue.Service, sprintSvc *sprint.Service, workflowSvc *workflow.Service, issueH *IssueHandler, chk *authz.Checker, baseURL string) *AgileBoardHandler {
	return &AgileBoardHandler{boardSvc: boardSvc, projectSvc: projectSvc, issueSvc: issueSvc, sprintSvc: sprintSvc, workflowSvc: workflowSvc, issueH: issueH, chk: chk, baseURL: baseURL}
}

// boardInputFor costruisce il BoardInput risolvendo il progetto della board.
func (h *AgileBoardHandler) boardInputFor(b *board.Board) v3.BoardInput {
	in := v3.BoardInput{SeqID: b.SeqID, Name: b.Name, Type: b.Type, BaseURL: h.baseURL}
	if p, err := h.projectSvc.GetByID(b.ProjectID); err == nil {
		in.ProjectID = p.SeqID
		in.ProjectKey = p.Key
		in.ProjectName = p.Name
		in.ProjectTypeKey = project.ProjectTypeKeyForType(p.Type)
	}
	return in
}

// resolveProject risolve un progetto da key o da id numerico (seq_id).
func (h *AgileBoardHandler) resolveProject(keyOrID string) (*project.Project, error) {
	if n, err := strconv.ParseInt(keyOrID, 10, 64); err == nil {
		return h.projectSvc.GetBySeqID(n)
	}
	return h.projectSvc.GetByKey(keyOrID)
}

// resolveBoard trova la board dal path param boardId (intero seq_id).
func (h *AgileBoardHandler) resolveBoard(r *http.Request) *board.Board {
	n, err := strconv.ParseInt(r.PathValue("boardId"), 10, 64)
	if err != nil {
		return nil
	}
	b, err := h.boardSvc.GetBySeqID(n)
	if err != nil {
		return nil
	}
	return b
}

func (h *AgileBoardHandler) List(w http.ResponseWriter, r *http.Request) {
	startAt, maxResults := v3.ParsePagination(r, 50, 100)
	boards, total, err := h.boardSvc.List(startAt, maxResults)
	if err != nil {
		v3.WriteError(w, http.StatusInternalServerError, []string{"failed to list boards"}, nil)
		return
	}
	values := make([]v3.Board, 0, len(boards))
	for i := range boards {
		values = append(values, v3.AgileBoard(h.boardInputFor(&boards[i])))
	}
	v3.WritePage(w, http.StatusOK, v3.Page[v3.Board]{StartAt: startAt, MaxResults: maxResults, Total: total, Values: values})
}

func (h *AgileBoardHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name           string `json:"name"`
		Type           string `json:"type"`
		ProjectKeyOrID string `json:"projectKeyOrId"` // scorciatoia nostra
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		v3.WriteError(w, http.StatusBadRequest, []string{"invalid request body"}, nil)
		return
	}
	if req.Name == "" || req.ProjectKeyOrID == "" {
		v3.WriteError(w, http.StatusBadRequest, []string{"name and projectKeyOrId are required"}, nil)
		return
	}
	p, err := h.resolveProject(req.ProjectKeyOrID)
	if err != nil {
		v3.WriteError(w, http.StatusNotFound, []string{"project not found"}, nil)
		return
	}
	uid := middleware.UserIDFromContext(r.Context())
	if err := h.chk.RequireProject(uid, p.ID, permission.AdministerProjects); err != nil {
		authz.WriteForbidden(w)
		return
	}
	if req.Type == "" {
		req.Type = "scrum"
	}
	b, err := h.boardSvc.Create(p.ID, req.Name, req.Type, nil)
	if err != nil {
		v3.WriteError(w, http.StatusInternalServerError, []string{"failed to create board"}, nil)
		return
	}
	v3.WriteJSON(w, http.StatusCreated, v3.AgileBoard(h.boardInputFor(b)))
}

func (h *AgileBoardHandler) Get(w http.ResponseWriter, r *http.Request) {
	b := h.resolveBoard(r)
	if b == nil {
		v3.WriteError(w, http.StatusNotFound, []string{"board not found"}, nil)
		return
	}
	v3.WriteJSON(w, http.StatusOK, v3.AgileBoard(h.boardInputFor(b)))
}

func (h *AgileBoardHandler) Delete(w http.ResponseWriter, r *http.Request) {
	b := h.resolveBoard(r)
	if b == nil {
		v3.WriteError(w, http.StatusNotFound, []string{"board not found"}, nil)
		return
	}
	if err := h.boardSvc.Delete(b.ID); err != nil {
		v3.WriteError(w, http.StatusInternalServerError, []string{"failed to delete board"}, nil)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *AgileBoardHandler) Configuration(w http.ResponseWriter, r *http.Request) {
	b := h.resolveBoard(r)
	if b == nil {
		v3.WriteError(w, http.StatusNotFound, []string{"board not found"}, nil)
		return
	}
	cfg := v3.BoardConfig{
		ID:   b.SeqID,
		Self: h.baseURL + "/rest/agile/1.0/board/" + strconv.FormatInt(b.SeqID, 10) + "/configuration",
		Name: b.Name,
		Type: b.Type,
	}
	cfg.ColumnConfig.ConstraintType = "none"
	if wf, err := h.workflowSvc.GetWorkflow(b.ProjectID); err == nil {
		sort.Slice(wf.Statuses, func(i, j int) bool { return wf.Statuses[i].Position < wf.Statuses[j].Position })
		for _, st := range wf.Statuses {
			cfg.ColumnConfig.Columns = append(cfg.ColumnConfig.Columns, v3.BoardColumnConfig{
				Name:     st.Name,
				Statuses: []v3.BoardColumnStatus{{ID: st.ID}},
			})
		}
	}
	v3.WriteJSON(w, http.StatusOK, cfg)
}

// writeIssuePage scrive una lista di issue nello shape SearchResults (issues+total).
func (h *AgileBoardHandler) writeIssuePage(w http.ResponseWriter, issues []issue.Issue, startAt, maxResults, total int) {
	items, err := renderIssueList(h.issueH, issues, nil)
	if err != nil {
		v3.WriteError(w, http.StatusInternalServerError, []string{"render error"}, nil)
		return
	}
	v3.WriteJSON(w, http.StatusOK, v3.SearchResults{Issues: items, StartAt: startAt, MaxResults: maxResults, Total: total})
}

// page applica la paginazione offset a uno slice di issue.
func page(issues []issue.Issue, startAt, maxResults int) []issue.Issue {
	if startAt > len(issues) {
		return []issue.Issue{}
	}
	end := startAt + maxResults
	if end > len(issues) {
		end = len(issues)
	}
	return issues[startAt:end]
}

// BoardIssues: tutte le issue del progetto della board.
func (h *AgileBoardHandler) BoardIssues(w http.ResponseWriter, r *http.Request) {
	b := h.resolveBoard(r)
	if b == nil {
		v3.WriteError(w, http.StatusNotFound, []string{"board not found"}, nil)
		return
	}
	all, err := h.issueSvc.ListByProject(b.ProjectID, issue.WithNotArchived())
	if err != nil {
		v3.WriteError(w, http.StatusInternalServerError, []string{"failed to list issues"}, nil)
		return
	}
	startAt, maxResults := v3.ParsePagination(r, 50, 100)
	h.writeIssuePage(w, page(all, startAt, maxResults), startAt, maxResults, len(all))
}

// Backlog: issue del progetto senza sprint (sprint_id IS NULL).
func (h *AgileBoardHandler) Backlog(w http.ResponseWriter, r *http.Request) {
	b := h.resolveBoard(r)
	if b == nil {
		v3.WriteError(w, http.StatusNotFound, []string{"board not found"}, nil)
		return
	}
	all, err := h.issueSvc.ListByProject(b.ProjectID, issue.WithNotArchived())
	if err != nil {
		v3.WriteError(w, http.StatusInternalServerError, []string{"failed to list issues"}, nil)
		return
	}
	var backlog []issue.Issue
	for _, iss := range all {
		if iss.SprintID == nil {
			backlog = append(backlog, iss)
		}
	}
	startAt, maxResults := v3.ParsePagination(r, 50, 100)
	h.writeIssuePage(w, page(backlog, startAt, maxResults), startAt, maxResults, len(backlog))
}

// BoardSprints: sprint del progetto della board (shape values+isLast).
func (h *AgileBoardHandler) BoardSprints(w http.ResponseWriter, r *http.Request) {
	b := h.resolveBoard(r)
	if b == nil {
		v3.WriteError(w, http.StatusNotFound, []string{"board not found"}, nil)
		return
	}
	sprints, err := h.sprintSvc.ListByProject(b.ProjectID)
	if err != nil {
		v3.WriteError(w, http.StatusInternalServerError, []string{"failed to list sprints"}, nil)
		return
	}
	startAt, maxResults := v3.ParsePagination(r, 50, 100)
	values := make([]v3.Sprint, 0, len(sprints))
	for i := range sprints {
		values = append(values, sprintToV3(&sprints[i], h.baseURL))
	}
	v3.WritePage(w, http.StatusOK, v3.Page[v3.Sprint]{StartAt: startAt, MaxResults: maxResults, Total: len(values), Values: values})
}

// BoardEpics: issue di tipo Epic nel progetto (shape values+isLast, minimal).
func (h *AgileBoardHandler) BoardEpics(w http.ResponseWriter, r *http.Request) {
	b := h.resolveBoard(r)
	if b == nil {
		v3.WriteError(w, http.StatusNotFound, []string{"board not found"}, nil)
		return
	}
	all, err := h.issueSvc.ListByProject(b.ProjectID, issue.WithNotArchived())
	if err != nil {
		v3.WriteError(w, http.StatusInternalServerError, []string{"failed to list issues"}, nil)
		return
	}
	epics := make([]map[string]any, 0)
	for i := range all {
		if h.issueH.isEpic(&all[i]) {
			epics = append(epics, map[string]any{
				"id":      all[i].SeqID,
				"key":     all[i].Key,
				"name":    all[i].Title,
				"summary": all[i].Title,
				"done":    false,
			})
		}
	}
	startAt, maxResults := v3.ParsePagination(r, 50, 100)
	lo := startAt
	if lo > len(epics) {
		lo = len(epics)
	}
	hi := lo + maxResults
	if hi > len(epics) {
		hi = len(epics)
	}
	v3.WriteJSON(w, http.StatusOK, map[string]any{
		"startAt": startAt, "maxResults": maxResults, "total": len(epics),
		"isLast": hi >= len(epics), "values": epics[lo:hi],
	})
}

// sprintToV3 mappa uno sprint di dominio nella Sprint agile.
func sprintToV3(sp *sprint.Sprint, baseURL string) v3.Sprint {
	return v3.AgileSprint(v3.SprintInput{
		SeqID: sp.SeqID, Name: sp.Name, State: string(sp.State), Goal: sp.Goal,
		OriginBoardID: sp.OriginBoardID, StartDate: sp.StartDate, EndDate: sp.EndDate,
		CompleteDate: sp.CompleteDate, BaseURL: baseURL,
	})
}
