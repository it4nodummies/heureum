package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/open-jira/open-jira/internal/domain/calendar"
)

type CalendarHandler struct {
	svc *calendar.Service
}

func NewCalendarHandler(svc *calendar.Service) *CalendarHandler {
	return &CalendarHandler{svc: svc}
}

func (h *CalendarHandler) GetCalendar(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("projectID")
	year, _ := strconv.Atoi(r.URL.Query().Get("year"))
	month, _ := strconv.Atoi(r.URL.Query().Get("month"))
	if year == 0 || month == 0 {
		http.Error(w, `{"error":"year and month query params required"}`, http.StatusBadRequest)
		return
	}
	data, _ := h.svc.GetCalendarData(projectID, year, month)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}
