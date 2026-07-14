package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/it4nodummies/heureum/internal/api/authz"
	"github.com/it4nodummies/heureum/internal/api/middleware"
	"github.com/it4nodummies/heureum/internal/domain/customfield"
	"github.com/it4nodummies/heureum/internal/domain/permission"
)

type CustomFieldHandler struct {
	svc *customfield.Service
	chk *authz.Checker
}

func NewCustomFieldHandler(svc *customfield.Service, chk *authz.Checker) *CustomFieldHandler {
	return &CustomFieldHandler{svc: svc, chk: chk}
}

func (h *CustomFieldHandler) ListFields(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("projectID")
	fields, _ := h.svc.ListFields(projectID)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(fields)
}

func (h *CustomFieldHandler) CreateField(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("projectID")
	var req struct {
		Name      string                `json:"name"`
		FieldType customfield.FieldType `json:"field_type"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}
	f, err := h.svc.CreateField(projectID, req.Name, req.FieldType)
	if err != nil {
		http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(f)
}

func (h *CustomFieldHandler) DeleteField(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.DeleteField(r.PathValue("fieldID")); err != nil {
		http.Error(w, `{"error":"failed to delete field"}`, http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *CustomFieldHandler) ListOptions(w http.ResponseWriter, r *http.Request) {
	opts, _ := h.svc.ListOptions(r.PathValue("fieldID"))
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(opts)
}

func (h *CustomFieldHandler) AddOption(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Value string `json:"value"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}
	o, err := h.svc.AddOption(r.PathValue("fieldID"), req.Value)
	if err != nil {
		http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(o)
}

// RemoveOption: two-hop authorization (option -> field -> project), enforced
// in-handler because DELETE /custom-fields/options/{optionID} has no single
// path-resolvable project (left unwrapped by the router decorator, per the
// Round 11 plan).
func (h *CustomFieldHandler) RemoveOption(w http.ResponseWriter, r *http.Request) {
	opt, err := h.svc.GetOption(r.PathValue("optionID"))
	if err != nil {
		http.Error(w, `{"error":"option not found"}`, http.StatusNotFound)
		return
	}
	field, err := h.svc.GetField(opt.FieldID)
	if err != nil {
		http.Error(w, `{"error":"option not found"}`, http.StatusNotFound)
		return
	}
	uid := middleware.UserIDFromContext(r.Context())
	if err := h.chk.RequireProject(uid, field.ProjectID, permission.AdministerProjects); err != nil {
		authz.WriteForbidden(w)
		return
	}
	if err := h.svc.RemoveOption(opt.ID); err != nil {
		http.Error(w, `{"error":"failed to remove option"}`, http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *CustomFieldHandler) SetValue(w http.ResponseWriter, r *http.Request) {
	issueID := r.PathValue("issueID")
	fieldID := r.PathValue("fieldID")
	var req struct {
		Value    interface{} `json:"value"`
		OptionID *string     `json:"option_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}
	if req.OptionID != nil && *req.OptionID != "" {
		if err := h.svc.SetOptionValue(issueID, fieldID, *req.OptionID); err != nil {
			http.Error(w, `{"error":"failed to set value"}`, http.StatusInternalServerError)
			return
		}
	} else {
		if err := h.svc.SetValue(issueID, fieldID, req.Value); err != nil {
			http.Error(w, `{"error":"failed to set value"}`, http.StatusInternalServerError)
			return
		}
	}
	w.WriteHeader(http.StatusOK)
}

func (h *CustomFieldHandler) GetValues(w http.ResponseWriter, r *http.Request) {
	issueID := r.PathValue("issueID")
	values, _ := h.svc.GetValues(issueID)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(values)
}
