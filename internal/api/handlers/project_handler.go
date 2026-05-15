package handlers

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/open-jira/open-jira/internal/domain/project"
	"github.com/open-jira/open-jira/internal/domain/workflow"
)

type ProjectHandler struct {
	svc   *project.Service
	wfSvc *workflow.Service
}

func NewProjectHandler(svc *project.Service, wfSvc *workflow.Service) *ProjectHandler {
	return &ProjectHandler{svc: svc, wfSvc: wfSvc}
}

func (h *ProjectHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name        string       `json:"name"`
		Key         string       `json:"key"`
		Description string       `json:"description"`
		Type        project.Type `json:"type"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}
	if req.Name == "" || req.Key == "" {
		http.Error(w, `{"error":"name and key are required"}`, http.StatusBadRequest)
		return
	}
	p, err := h.svc.Create(req.Name, req.Key, req.Description, req.Type)
	if err != nil {
		http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusConflict)
		return
	}
	if _, err := h.wfSvc.CreateDefaultWorkflow(p.ID); err != nil {
		log.Printf("failed to create default workflow for project %s: %v", p.Key, err)
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(p)
}

func (h *ProjectHandler) Get(w http.ResponseWriter, r *http.Request) {
	p, err := h.svc.GetByKey(r.PathValue("key"))
	if err != nil {
		http.Error(w, `{"error":"project not found"}`, http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(p)
}

func (h *ProjectHandler) List(w http.ResponseWriter, r *http.Request) {
	projects, _ := h.svc.List(false)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(projects)
}

func (h *ProjectHandler) Update(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	p, err := h.svc.Update(r.PathValue("key"), req.Name, req.Description)
	if err != nil {
		http.Error(w, `{"error":"project not found"}`, http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(p)
}

func (h *ProjectHandler) Delete(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.Archive(r.PathValue("key")); err != nil {
		http.Error(w, `{"error":"project not found"}`, http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *ProjectHandler) ListMembers(w http.ResponseWriter, r *http.Request) {
	p, err := h.svc.GetByKey(r.PathValue("key"))
	if err != nil {
		http.Error(w, `{"error":"project not found"}`, http.StatusNotFound)
		return
	}
	members, _ := h.svc.ListMembers(p.ID)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(members)
}

func (h *ProjectHandler) AddMember(w http.ResponseWriter, r *http.Request) {
	p, err := h.svc.GetByKey(r.PathValue("key"))
	if err != nil {
		http.Error(w, `{"error":"project not found"}`, http.StatusNotFound)
		return
	}
	var req struct {
		UserID string             `json:"user_id"`
		Role   project.MemberRole `json:"role"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	if err := h.svc.AddMember(p.ID, req.UserID, req.Role); err != nil {
		http.Error(w, `{"error":"failed to add member"}`, http.StatusConflict)
		return
	}
	w.WriteHeader(http.StatusCreated)
}

func (h *ProjectHandler) RemoveMember(w http.ResponseWriter, r *http.Request) {
	p, err := h.svc.GetByKey(r.PathValue("key"))
	if err != nil {
		http.Error(w, `{"error":"project not found"}`, http.StatusNotFound)
		return
	}
	h.svc.RemoveMember(p.ID, r.PathValue("userId"))
	w.WriteHeader(http.StatusNoContent)
}

func (h *ProjectHandler) Invite(w http.ResponseWriter, r *http.Request) {
	p, err := h.svc.GetByKey(r.PathValue("key"))
	if err != nil {
		http.Error(w, `{"error":"project not found"}`, http.StatusNotFound)
		return
	}
	var req struct {
		Email string             `json:"email"`
		Role  project.MemberRole `json:"role"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	inv, err := project.CreateInvite(h.svc.DB(), p.ID, req.Email, req.Role)
	if err != nil {
		http.Error(w, `{"error":"failed to create invite"}`, http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{"token": inv.Token})
}
