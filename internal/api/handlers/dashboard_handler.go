package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/it4nodummies/heureum/internal/api/middleware"
	"github.com/it4nodummies/heureum/internal/domain/dashboard"
)

type DashboardHandler struct {
	svc *dashboard.Service
}

func NewDashboardHandler(svc *dashboard.Service) *DashboardHandler {
	return &DashboardHandler{svc: svc}
}

func (h *DashboardHandler) List(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	dashboards, _ := h.svc.ListDashboards(userID)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(dashboards)
}

func (h *DashboardHandler) Create(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	var req struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}
	if req.Name == "" {
		http.Error(w, `{"error":"name is required"}`, http.StatusBadRequest)
		return
	}
	d, err := h.svc.CreateDashboard(userID, req.Name)
	if err != nil {
		http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(d)
}

func (h *DashboardHandler) Get(w http.ResponseWriter, r *http.Request) {
	d, err := h.svc.GetDashboard(r.PathValue("id"))
	if err != nil {
		http.Error(w, `{"error":"dashboard not found"}`, http.StatusNotFound)
		return
	}
	widgets, _ := h.svc.GetWidgets(d.ID)

	type widgetJSON struct {
		ID           string          `json:"id"`
		DashboardID  string          `json:"dashboard_id"`
		WidgetType   string          `json:"widget_type"`
		ConfigJSON   string          `json:"config_json"`
		PositionJSON string          `json:"position_json"`
		Data         interface{}     `json:"data,omitempty"`
	}
	wjs := make([]widgetJSON, 0, len(widgets))
	for _, w := range widgets {
		wj := widgetJSON{
			ID:           w.ID,
			DashboardID:  w.DashboardID,
			WidgetType:   w.WidgetType,
			ConfigJSON:   w.ConfigJSON,
			PositionJSON: w.PositionJSON,
		}
		userID := middleware.UserIDFromContext(r.Context())
		switch w.WidgetType {
		case "assigned_to_me":
			issues, _ := h.svc.GetAssignedIssues(userID)
			wj.Data = issues
		case "activity_stream":
			activity, _ := h.svc.GetActivityFeed(userID, 10)
			wj.Data = activity
		}
		wjs = append(wjs, wj)
	}

	type dashboardResponse struct {
		dashboard.Dashboard
		Widgets []widgetJSON `json:"widgets"`
	}

	resp := dashboardResponse{Dashboard: *d, Widgets: wjs}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (h *DashboardHandler) Update(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name       *string `json:"name"`
		IsPublic   *bool   `json:"is_public"`
		LayoutJSON *string `json:"layout_json"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}
	name := ""
	if req.Name != nil {
		name = *req.Name
	}
	d, err := h.svc.UpdateDashboard(r.PathValue("id"), name, req.IsPublic, req.LayoutJSON)
	if err != nil {
		http.Error(w, `{"error":"dashboard not found"}`, http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(d)
}

func (h *DashboardHandler) Delete(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.DeleteDashboard(r.PathValue("id")); err != nil {
		http.Error(w, `{"error":"dashboard not found"}`, http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *DashboardHandler) AddWidget(w http.ResponseWriter, r *http.Request) {
	var req struct {
		WidgetType string `json:"widget_type"`
		ConfigJSON string `json:"config_json"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}
	if req.WidgetType == "" {
		http.Error(w, `{"error":"widget_type is required"}`, http.StatusBadRequest)
		return
	}
	if req.ConfigJSON == "" {
		req.ConfigJSON = "{}"
	}
	widget, err := h.svc.AddWidget(r.PathValue("id"), req.WidgetType, req.ConfigJSON)
	if err != nil {
		http.Error(w, `{"error":"dashboard not found"}`, http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(widget)
}

func (h *DashboardHandler) RemoveWidget(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.RemoveWidget(r.PathValue("widgetId")); err != nil {
		http.Error(w, `{"error":"widget not found"}`, http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *DashboardHandler) RemoveGadget(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.RemoveWidget(r.PathValue("gadgetId")); err != nil {
		http.Error(w, `{"error":"widget not found"}`, http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *DashboardHandler) SearchDashboards(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	dashboards, _ := h.svc.ListDashboards(userID)

	query := r.URL.Query().Get("dashboardName")
	if query != "" {
		filtered := make([]dashboard.Dashboard, 0)
		lowerQuery := query
		for _, d := range dashboards {
			if len(d.Name) > 0 {
				if len(d.Name) >= len(lowerQuery) {
					if d.Name == lowerQuery || containsIgnoreCase(d.Name, lowerQuery) {
						filtered = append(filtered, d)
					}
				}
			}
		}
		dashboards = filtered
	}
	if dashboards == nil {
		dashboards = []dashboard.Dashboard{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"values": dashboards,
		"total":  len(dashboards),
	})
}

func containsIgnoreCase(s, substr string) bool {
	if len(substr) > len(s) {
		return false
	}
	for i := 0; i <= len(s)-len(substr); i++ {
		match := true
		for j := 0; j < len(substr); j++ {
			sc := s[i+j]
			ss := substr[j]
			if sc >= 'A' && sc <= 'Z' {
				sc += 32
			}
			if ss >= 'A' && ss <= 'Z' {
				ss += 32
			}
			if sc != ss {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}

func (h *DashboardHandler) CopyDashboard(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	original, err := h.svc.GetDashboard(r.PathValue("id"))
	if err != nil {
		http.Error(w, `{"error":"dashboard not found"}`, http.StatusNotFound)
		return
	}
	copied, err := h.svc.CreateDashboard(userID, original.Name+" (copy)")
	if err != nil {
		http.Error(w, `{"error":"failed to copy dashboard"}`, http.StatusInternalServerError)
		return
	}
	widgets, _ := h.svc.GetWidgets(original.ID)
	for _, wg := range widgets {
		h.svc.AddWidget(copied.ID, wg.WidgetType, wg.ConfigJSON)
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(copied)
}
