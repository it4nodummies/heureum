package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/it4nodummies/heureum/internal/domain/project"
	"github.com/it4nodummies/heureum/internal/domain/timeline"
)

type TimelineHandler struct {
	svc        *timeline.Service
	projectSvc *project.Service
}

func NewTimelineHandler(svc *timeline.Service, projectSvc *project.Service) *TimelineHandler {
	return &TimelineHandler{svc: svc, projectSvc: projectSvc}
}

// GetTimeline serves GET /rest/api/3/project/{key}/timeline. {key} is the
// public project key/seq_id; the domain service keys on the internal UUID, so
// resolve it here. zoom is one of weeks|months|quarters (default weeks).
func (h *TimelineHandler) GetTimeline(w http.ResponseWriter, r *http.Request) {
	p, err := h.projectSvc.GetByKey(r.PathValue("key"))
	if err != nil {
		http.Error(w, `{"error":"project not found"}`, http.StatusNotFound)
		return
	}
	zoom := r.URL.Query().Get("zoom")
	if zoom == "" {
		zoom = "weeks"
	}
	data, err := h.svc.GetTimelineData(p.ID, zoom)
	if err != nil {
		http.Error(w, `{"error":"timeline failed"}`, http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}
