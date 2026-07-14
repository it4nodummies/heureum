package handlers

import (
	"encoding/json"
	"net/http"
	"sort"
	"strconv"

	v3 "github.com/open-jira/open-jira/internal/api/v3"
	"github.com/open-jira/open-jira/internal/domain/board"
	"github.com/open-jira/open-jira/internal/domain/issue"
	"github.com/open-jira/open-jira/internal/domain/project"
	"github.com/open-jira/open-jira/internal/domain/sprint"
	"github.com/open-jira/open-jira/internal/domain/workflow"
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
	baseURL     string
}

func NewAgileBoardHandler(boardSvc *board.Service, projectSvc *project.Service, issueSvc *issue.Service, sprintSvc *sprint.Service, workflowSvc *workflow.Service, issueH *IssueHandler, baseURL string) *AgileBoardHandler {
	return &AgileBoardHandler{boardSvc: boardSvc, projectSvc: projectSvc, issueSvc: issueSvc, sprintSvc: sprintSvc, workflowSvc: workflowSvc, issueH: issueH, baseURL: baseURL}
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
