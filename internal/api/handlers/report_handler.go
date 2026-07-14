package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/it4nodummies/heureum/internal/api/v3"
	"github.com/it4nodummies/heureum/internal/domain/project"
	"github.com/it4nodummies/heureum/internal/domain/report"
)

type ReportHandler struct {
	svc        *report.Service
	projectSvc *project.Service
}

func NewReportHandler(svc *report.Service, projectSvc *project.Service) *ReportHandler {
	return &ReportHandler{svc: svc, projectSvc: projectSvc}
}

func (h *ReportHandler) Burndown(w http.ResponseWriter, r *http.Request) {
	p, err := h.projectSvc.GetByKey(r.PathValue("key"))
	if err != nil {
		http.Error(w, `{"error":"project not found"}`, http.StatusNotFound)
		return
	}
	_ = p
	sprintID := r.URL.Query().Get("sprintId")
	if sprintID == "" {
		http.Error(w, `{"error":"sprintId query parameter is required"}`, http.StatusBadRequest)
		return
	}
	data, err := h.svc.GetBurndownData(sprintID)
	if err != nil {
		http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}

func (h *ReportHandler) Velocity(w http.ResponseWriter, r *http.Request) {
	p, err := h.projectSvc.GetByKey(r.PathValue("key"))
	if err != nil {
		http.Error(w, `{"error":"project not found"}`, http.StatusNotFound)
		return
	}
	data, err := h.svc.GetVelocity(p.ID)
	if err != nil {
		http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}

func (h *ReportHandler) Burnup(w http.ResponseWriter, r *http.Request) {
	p, err := h.projectSvc.GetByKey(r.PathValue("key"))
	if err != nil {
		http.Error(w, `{"error":"project not found"}`, http.StatusNotFound)
		return
	}
	_ = p
	sprintID := r.URL.Query().Get("sprintId")
	if sprintID == "" {
		http.Error(w, `{"error":"sprintId query parameter is required"}`, http.StatusBadRequest)
		return
	}
	data, err := h.svc.GetBurnupData(sprintID)
	if err != nil {
		http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}

func (h *ReportHandler) CFD(w http.ResponseWriter, r *http.Request) {
	p, err := h.projectSvc.GetByKey(r.PathValue("key"))
	if err != nil {
		http.Error(w, `{"error":"project not found"}`, http.StatusNotFound)
		return
	}
	data, err := h.svc.GetCFD(p.ID)
	if err != nil {
		http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}

// Pie: GET /rest/api/3/project/{key}/reports/pie?field=status|priority|assignee|type
func (h *ReportHandler) Pie(w http.ResponseWriter, r *http.Request) {
	p, err := h.projectSvc.GetByKey(r.PathValue("key"))
	if err != nil {
		v3.WriteError(w, http.StatusNotFound, []string{"project not found"}, nil)
		return
	}
	field := r.URL.Query().Get("field")
	if field == "" {
		field = "status"
	}
	slices, err := h.svc.GetPieByField(p.ID, field)
	if err != nil {
		v3.WriteError(w, http.StatusBadRequest, []string{err.Error()}, nil)
		return
	}
	v3.WriteJSON(w, http.StatusOK, slices)
}

func (h *ReportHandler) Summary(w http.ResponseWriter, r *http.Request) {
	p, err := h.projectSvc.GetByKey(r.PathValue("key"))
	if err != nil {
		http.Error(w, `{"error":"project not found"}`, http.StatusNotFound)
		return
	}
	data, err := h.svc.GetProjectSummary(p.ID)
	if err != nil {
		http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}

// CreatedVsResolved: GET /rest/api/3/project/{key}/reports/created-vs-resolved?days=30
func (h *ReportHandler) CreatedVsResolved(w http.ResponseWriter, r *http.Request) {
	p, err := h.projectSvc.GetByKey(r.PathValue("key"))
	if err != nil {
		v3.WriteError(w, http.StatusNotFound, []string{"project not found"}, nil)
		return
	}
	days := 30
	if v := r.URL.Query().Get("days"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			days = n
		}
	}
	data, err := h.svc.GetCreatedVsResolved(p.ID, days)
	if err != nil {
		v3.WriteError(w, http.StatusInternalServerError, []string{"failed to compute report"}, nil)
		return
	}
	v3.WriteJSON(w, http.StatusOK, data)
}
