package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/it4nodummies/heureum/internal/domain/timeline"
)

type TimelineHandler struct {
	svc *timeline.Service
}

func NewTimelineHandler(svc *timeline.Service) *TimelineHandler {
	return &TimelineHandler{svc: svc}
}

func (h *TimelineHandler) GetTimeline(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("projectID")
	zoom := r.URL.Query().Get("zoom")
	if zoom == "" {
		zoom = "weeks"
	}
	data, _ := h.svc.GetTimelineData(projectID, zoom)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}
