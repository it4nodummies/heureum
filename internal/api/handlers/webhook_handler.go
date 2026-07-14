package handlers

import (
	"encoding/json"
	"net/http"

	v3 "github.com/it4nodummies/heureum/internal/api/v3"
	"github.com/it4nodummies/heureum/internal/domain/project"
	"github.com/it4nodummies/heureum/internal/domain/webhook"
)

type WebhookHandler struct {
	svc        *webhook.Service
	projectSvc *project.Service
}

func NewWebhookHandler(svc *webhook.Service, projectSvc *project.Service) *WebhookHandler {
	return &WebhookHandler{svc: svc, projectSvc: projectSvc}
}

// webhookOut è la rappresentazione di risposta (events come array, secret nascosto).
type webhookOut struct {
	ID        string   `json:"id"`
	ProjectID string   `json:"project_id"`
	URL       string   `json:"url"`
	Events    []string `json:"events"`
	IsActive  bool     `json:"is_active"`
}

func toOut(h webhook.Webhook) webhookOut {
	return webhookOut{ID: h.ID, ProjectID: h.ProjectID, URL: h.URL, Events: h.Events(), IsActive: h.IsActive}
}

// List: GET /rest/api/3/project/{key}/webhooks.
func (h *WebhookHandler) List(w http.ResponseWriter, r *http.Request) {
	p, err := h.projectSvc.GetByKey(r.PathValue("key"))
	if err != nil {
		v3.WriteError(w, http.StatusNotFound, []string{"project not found"}, nil)
		return
	}
	hooks, err := h.svc.ListByProject(p.ID)
	if err != nil {
		v3.WriteError(w, http.StatusInternalServerError, []string{"failed to list webhooks"}, nil)
		return
	}
	out := make([]webhookOut, 0, len(hooks))
	for _, hook := range hooks {
		out = append(out, toOut(hook))
	}
	v3.WriteJSON(w, http.StatusOK, out)
}

// Create: POST /rest/api/3/project/{key}/webhooks {url, secret?, events[]}.
func (h *WebhookHandler) Create(w http.ResponseWriter, r *http.Request) {
	p, err := h.projectSvc.GetByKey(r.PathValue("key"))
	if err != nil {
		v3.WriteError(w, http.StatusNotFound, []string{"project not found"}, nil)
		return
	}
	var req struct {
		URL    string   `json:"url"`
		Secret string   `json:"secret"`
		Events []string `json:"events"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.URL == "" {
		v3.WriteError(w, http.StatusBadRequest, []string{"url is required"}, nil)
		return
	}
	if len(req.Events) == 0 {
		req.Events = []string{"issue_created", "issue_updated", "issue_transitioned"}
	}
	hook, err := h.svc.Create(p.ID, req.URL, req.Secret, req.Events)
	if err != nil {
		v3.WriteError(w, http.StatusInternalServerError, []string{"failed to create webhook"}, nil)
		return
	}
	v3.WriteJSON(w, http.StatusCreated, toOut(*hook))
}

// Delete: DELETE /rest/api/3/project/{key}/webhooks/{id}.
func (h *WebhookHandler) Delete(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.Delete(r.PathValue("id")); err != nil {
		v3.WriteError(w, http.StatusInternalServerError, []string{"failed to delete webhook"}, nil)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
