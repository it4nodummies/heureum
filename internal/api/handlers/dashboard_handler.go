package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/it4nodummies/heureum/internal/api/authz"
	"github.com/it4nodummies/heureum/internal/api/middleware"
	"github.com/it4nodummies/heureum/internal/domain/dashboard"
)

type DashboardHandler struct {
	svc *dashboard.Service
	chk *authz.Checker
}

func NewDashboardHandler(svc *dashboard.Service, chk *authz.Checker) *DashboardHandler {
	return &DashboardHandler{svc: svc, chk: chk}
}

// pathDashboardID legge l'id del dashboard dal path, gestendo entrambi gli
// alias di rotta esistenti (/dashboards/{id} e /dashboard/{dashboardId}/gadget).
func pathDashboardID(r *http.Request) string {
	if id := r.PathValue("id"); id != "" {
		return id
	}
	return r.PathValue("dashboardId")
}

// requireDashboardOwner carica il dashboard id e verifica che l'utente
// autenticato ne sia il proprietario (o admin globale). Scrive 404 se il
// dashboard non esiste, 403 se esiste ma appartiene a un altro utente.
func (h *DashboardHandler) requireDashboardOwner(w http.ResponseWriter, r *http.Request, id string) (*dashboard.Dashboard, bool) {
	d, err := h.svc.GetDashboard(id)
	if err != nil {
		http.Error(w, `{"error":"dashboard not found"}`, http.StatusNotFound)
		return nil, false
	}
	uid := middleware.UserIDFromContext(r.Context())
	if d.OwnerID != uid && !h.chk.IsGlobalAdmin(uid) {
		authz.WriteForbidden(w)
		return nil, false
	}
	return d, true
}

// requireDashboardVisible carica il dashboard id e verifica che l'utente
// autenticato possa vederlo: proprietario, dashboard pubblica, o admin
// globale. Scrive 404 (senza rivelarne l'esistenza) se il dashboard non
// esiste o non è visibile.
func (h *DashboardHandler) requireDashboardVisible(w http.ResponseWriter, r *http.Request, id string) (*dashboard.Dashboard, bool) {
	d, err := h.svc.GetDashboard(id)
	if err != nil {
		http.Error(w, `{"error":"dashboard not found"}`, http.StatusNotFound)
		return nil, false
	}
	uid := middleware.UserIDFromContext(r.Context())
	if d.OwnerID != uid && !d.IsPublic && !h.chk.IsGlobalAdmin(uid) {
		http.Error(w, `{"error":"the resource does not exist or you do not have permission to view it"}`, http.StatusNotFound)
		return nil, false
	}
	return d, true
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
	d, ok := h.requireDashboardVisible(w, r, r.PathValue("id"))
	if !ok {
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
	id := r.PathValue("id")
	if _, ok := h.requireDashboardOwner(w, r, id); !ok {
		return
	}
	d, err := h.svc.UpdateDashboard(id, name, req.IsPublic, req.LayoutJSON)
	if err != nil {
		http.Error(w, `{"error":"dashboard not found"}`, http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(d)
}

func (h *DashboardHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if _, ok := h.requireDashboardOwner(w, r, id); !ok {
		return
	}
	if err := h.svc.DeleteDashboard(id); err != nil {
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
	dashboardID := pathDashboardID(r)
	if _, ok := h.requireDashboardOwner(w, r, dashboardID); !ok {
		return
	}
	widget, err := h.svc.AddWidget(dashboardID, req.WidgetType, req.ConfigJSON)
	if err != nil {
		http.Error(w, `{"error":"dashboard not found"}`, http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(widget)
}

func (h *DashboardHandler) RemoveWidget(w http.ResponseWriter, r *http.Request) {
	h.removeWidget(w, r, r.PathValue("widgetId"))
}

func (h *DashboardHandler) RemoveGadget(w http.ResponseWriter, r *http.Request) {
	h.removeWidget(w, r, r.PathValue("gadgetId"))
}

// removeWidget risolve il dashboard genitore del widget e verifica che
// l'utente autenticato ne sia il proprietario (o admin globale) prima di
// rimuovere il widget.
func (h *DashboardHandler) removeWidget(w http.ResponseWriter, r *http.Request, widgetID string) {
	widget, err := h.svc.GetWidget(widgetID)
	if err != nil {
		http.Error(w, `{"error":"widget not found"}`, http.StatusNotFound)
		return
	}
	if _, ok := h.requireDashboardOwner(w, r, widget.DashboardID); !ok {
		return
	}
	if err := h.svc.RemoveWidget(widgetID); err != nil {
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
	original, ok := h.requireDashboardVisible(w, r, r.PathValue("id"))
	if !ok {
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
