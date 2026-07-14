package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/it4nodummies/heureum/internal/domain/automation"
)

type AutomationHandler struct {
	svc *automation.Service
}

func NewAutomationHandler(svc *automation.Service) *AutomationHandler {
	return &AutomationHandler{svc: svc}
}

func (h *AutomationHandler) ListRules(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("projectID")
	rules, _ := h.svc.ListRules(projectID)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(rules)
}

func (h *AutomationHandler) CreateRule(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("projectID")
	var req struct {
		Name           string `json:"name"`
		TriggerType    string `json:"trigger_type"`
		ConditionsJSON string `json:"conditions_json"`
		ActionsJSON    string `json:"actions_json"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}
	rule, err := h.svc.CreateRule(projectID, req.Name, req.TriggerType, req.ConditionsJSON, req.ActionsJSON)
	if err != nil {
		http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(rule)
}

func (h *AutomationHandler) GetRule(w http.ResponseWriter, r *http.Request) {
	rule, err := h.svc.GetRule(r.PathValue("ruleID"))
	if err != nil {
		http.Error(w, `{"error":"automation rule not found"}`, http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(rule)
}

func (h *AutomationHandler) UpdateRule(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name           *string `json:"name"`
		IsActive       *bool   `json:"is_active"`
		TriggerType    *string `json:"trigger_type"`
		ConditionsJSON *string `json:"conditions_json"`
		ActionsJSON    *string `json:"actions_json"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	rule, err := h.svc.UpdateRule(r.PathValue("ruleID"), req.Name, req.IsActive, req.TriggerType, req.ConditionsJSON, req.ActionsJSON)
	if err != nil {
		http.Error(w, `{"error":"automation rule not found"}`, http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(rule)
}

func (h *AutomationHandler) DeleteRule(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.DeleteRule(r.PathValue("ruleID")); err != nil {
		http.Error(w, `{"error":"failed to delete rule"}`, http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *AutomationHandler) ExecuteRule(w http.ResponseWriter, r *http.Request) {
	var req struct {
		IssueID string `json:"issue_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}
	run, err := h.svc.TestRule(r.PathValue("ruleID"), req.IssueID)
	if err != nil {
		http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(run)
}

func (h *AutomationHandler) ListRuns(w http.ResponseWriter, r *http.Request) {
	runs, _ := h.svc.ListRuns(r.PathValue("ruleID"))
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(runs)
}
