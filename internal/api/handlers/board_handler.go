package handlers

import (
	"encoding/json"
	"math"
	"net/http"
	"sort"

	"github.com/it4nodummies/heureum/internal/api/authz"
	"github.com/it4nodummies/heureum/internal/api/middleware"
	"github.com/it4nodummies/heureum/internal/domain/issue"
	"github.com/it4nodummies/heureum/internal/domain/permission"
	"github.com/it4nodummies/heureum/internal/domain/project"
	"github.com/it4nodummies/heureum/internal/domain/workflow"
)

type BoardHandler struct {
	issueSvc    *issue.Service
	projectSvc  *project.Service
	workflowSvc *workflow.Service
	chk         *authz.Checker
}

func NewBoardHandler(issueSvc *issue.Service, projectSvc *project.Service, workflowSvc *workflow.Service, chk *authz.Checker) *BoardHandler {
	return &BoardHandler{issueSvc: issueSvc, projectSvc: projectSvc, workflowSvc: workflowSvc, chk: chk}
}

type BoardColumn struct {
	ID     string        `json:"id"`
	Name   string        `json:"name"`
	Issues []issue.Issue `json:"issues"`
}

func (h *BoardHandler) GetBoard(w http.ResponseWriter, r *http.Request) {
	p, err := h.projectSvc.GetByKey(r.PathValue("key"))
	if err != nil {
		http.Error(w, `{"error":"project not found"}`, http.StatusNotFound)
		return
	}
	wf, err := h.workflowSvc.GetWorkflow(p.ID)
	if err != nil {
		http.Error(w, `{"error":"workflow not found"}`, http.StatusNotFound)
		return
	}
	issues, _ := h.issueSvc.ListByProject(p.ID, issue.WithNotArchived())

	issuesByStatus := map[string][]issue.Issue{}
	for _, iss := range issues {
		sid := ""
		if iss.StatusID != nil {
			sid = *iss.StatusID
		}
		issuesByStatus[sid] = append(issuesByStatus[sid], iss)
	}

	sort.Slice(wf.Statuses, func(i, j int) bool {
		return wf.Statuses[i].Position < wf.Statuses[j].Position
	})

	var columns []BoardColumn
	for _, status := range wf.Statuses {
		colIssues := issuesByStatus[status.ID]
		if colIssues == nil {
			colIssues = []issue.Issue{}
		}
		sort.Slice(colIssues, func(i, j int) bool {
			return colIssues[i].Position < colIssues[j].Position
		})
		columns = append(columns, BoardColumn{
			ID:     status.ID,
			Name:   status.Name,
			Issues: colIssues,
		})
	}

	if unassigned := issuesByStatus[""]; len(unassigned) > 0 {
		columns = append(columns, BoardColumn{
			ID:     "",
			Name:   "Unassigned",
			Issues: unassigned,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"columns": columns})
}

func (h *BoardHandler) RankIssue(w http.ResponseWriter, r *http.Request) {
	var req struct {
		IssueID  string   `json:"issue_id"`
		AfterID  *string  `json:"after_id"`
		Position *float64 `json:"position"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}
	if req.IssueID == "" {
		http.Error(w, `{"error":"issue_id is required"}`, http.StatusBadRequest)
		return
	}

	// project resolved from the target issue (body: issue_id) -> MANAGE_SPRINTS.
	var targetIss issue.Issue
	if err := h.issueSvc.DB().First(&targetIss, "id = ?", req.IssueID).Error; err != nil {
		http.Error(w, `{"error":"issue not found"}`, http.StatusNotFound)
		return
	}
	uid := middleware.UserIDFromContext(r.Context())
	if err := h.chk.RequireProject(uid, targetIss.ProjectID, permission.ManageSprints); err != nil {
		authz.WriteForbidden(w)
		return
	}

	var newPos float64

	if req.Position != nil && *req.Position == 0 {
		db := h.issueSvc.DB()
		var afterIss issue.Issue
		if err := db.Order("position ASC").First(&afterIss).Error; err == nil {
			newPos = afterIss.Position / 2
		} else {
			newPos = 1000
		}
	} else if req.AfterID != nil {
		db := h.issueSvc.DB()
		var afterIss issue.Issue
		if err := db.Where("id = ?", *req.AfterID).First(&afterIss).Error; err != nil {
			http.Error(w, `{"error":"after issue not found"}`, http.StatusNotFound)
			return
		}
		var beforeIss issue.Issue
		if err := db.Where("position > ?", afterIss.Position).Order("position ASC").First(&beforeIss).Error; err == nil {
			newPos = (afterIss.Position + beforeIss.Position) / 2
		} else {
			newPos = afterIss.Position + 1000
		}
	} else {
		newPos = math.MaxFloat64 / 2
	}

	if err := h.issueSvc.DB().Table("issues").Where("id = ?", req.IssueID).Update("position", newPos).Error; err != nil {
		http.Error(w, `{"error":"failed to update position"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]float64{"position": newPos})
}
