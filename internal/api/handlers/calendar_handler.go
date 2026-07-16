package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/it4nodummies/heureum/internal/domain/calendar"
	"github.com/it4nodummies/heureum/internal/domain/project"
)

type CalendarHandler struct {
	svc        *calendar.Service
	projectSvc *project.Service
}

func NewCalendarHandler(svc *calendar.Service, projectSvc *project.Service) *CalendarHandler {
	return &CalendarHandler{svc: svc, projectSvc: projectSvc}
}

// GetCalendar serves GET /rest/api/3/project/{key}/calendar?year=&month=.
// {key} is resolved to the internal UUID the domain service expects.
func (h *CalendarHandler) GetCalendar(w http.ResponseWriter, r *http.Request) {
	p, err := h.projectSvc.GetByKey(r.PathValue("key"))
	if err != nil {
		http.Error(w, `{"error":"project not found"}`, http.StatusNotFound)
		return
	}
	year, _ := strconv.Atoi(r.URL.Query().Get("year"))
	month, _ := strconv.Atoi(r.URL.Query().Get("month"))
	if year == 0 || month == 0 {
		http.Error(w, `{"error":"year and month query params required"}`, http.StatusBadRequest)
		return
	}
	data, err := h.svc.GetCalendarData(p.ID, year, month)
	if err != nil {
		http.Error(w, `{"error":"calendar failed"}`, http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}
