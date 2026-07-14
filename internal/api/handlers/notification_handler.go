package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/it4nodummies/heureum/internal/api/middleware"
	"github.com/it4nodummies/heureum/internal/domain/notification"
)

type NotificationHandler struct {
	svc *notification.Service
}

func NewNotificationHandler(svc *notification.Service) *NotificationHandler {
	return &NotificationHandler{svc: svc}
}

func (h *NotificationHandler) List(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	unreadOnly := r.URL.Query().Get("unread") == "true"
	notifs, err := h.svc.ListByUser(userID, unreadOnly)
	if err != nil {
		http.Error(w, `{"error":"failed to fetch notifications"}`, http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(notifs)
}

func (h *NotificationHandler) MarkRead(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.MarkRead(r.PathValue("id")); err != nil {
		http.Error(w, `{"error":"notification not found"}`, http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *NotificationHandler) MarkAllRead(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	if err := h.svc.MarkAllRead(userID); err != nil {
		http.Error(w, `{"error":"failed to mark all read"}`, http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *NotificationHandler) UnreadCount(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	count, err := h.svc.GetUnreadCount(userID)
	if err != nil {
		http.Error(w, `{"error":"failed to get unread count"}`, http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]int64{"count": count})
}

func (h *NotificationHandler) GetSettings(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	settings, err := h.svc.GetSettings(userID)
	if err != nil {
		http.Error(w, `{"error":"failed to fetch settings"}`, http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(settings)
}

func (h *NotificationHandler) UpdateSettings(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	var req struct {
		ProjectID string `json:"project_id"`
		EventType string `json:"event_type"`
		ViaEmail  bool   `json:"via_email"`
		ViaApp    bool   `json:"via_app"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}
	if req.EventType == "" {
		http.Error(w, `{"error":"event_type is required"}`, http.StatusBadRequest)
		return
	}
	if err := h.svc.UpdateSetting(userID, req.ProjectID, req.EventType, req.ViaEmail, req.ViaApp); err != nil {
		http.Error(w, `{"error":"failed to update settings"}`, http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}
