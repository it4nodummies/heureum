package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/it4nodummies/heureum/internal/domain/project"
	"github.com/it4nodummies/heureum/internal/domain/sprint"
)

type SprintHandler struct {
	svc        *sprint.Service
	projectSvc *project.Service
}

func NewSprintHandler(svc *sprint.Service, projectSvc *project.Service) *SprintHandler {
	return &SprintHandler{svc: svc, projectSvc: projectSvc}
}

func (h *SprintHandler) List(w http.ResponseWriter, r *http.Request) {
	p, err := h.projectSvc.GetByKey(r.PathValue("key"))
	if err != nil {
		http.Error(w, `{"error":"project not found"}`, http.StatusNotFound)
		return
	}
	sprints, _ := h.svc.ListByProject(p.ID)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(sprints)
}

func (h *SprintHandler) Create(w http.ResponseWriter, r *http.Request) {
	p, err := h.projectSvc.GetByKey(r.PathValue("key"))
	if err != nil {
		http.Error(w, `{"error":"project not found"}`, http.StatusNotFound)
		return
	}
	var req struct {
		Name string `json:"name"`
		Goal string `json:"goal"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}
	if req.Name == "" {
		http.Error(w, `{"error":"name is required"}`, http.StatusBadRequest)
		return
	}
	sp, err := h.svc.Create(p.ID, req.Name, req.Goal)
	if err != nil {
		http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(sp)
}

func (h *SprintHandler) Get(w http.ResponseWriter, r *http.Request) {
	sp, err := h.svc.GetByID(r.PathValue("id"))
	if err != nil {
		http.Error(w, `{"error":"sprint not found"}`, http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(sp)
}

func (h *SprintHandler) Update(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name string `json:"name"`
		Goal string `json:"goal"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	sp, err := h.svc.Update(r.PathValue("id"), req.Name, req.Goal)
	if err != nil {
		http.Error(w, `{"error":"sprint not found"}`, http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(sp)
}

func (h *SprintHandler) Start(w http.ResponseWriter, r *http.Request) {
	sp, err := h.svc.Start(r.PathValue("id"))
	if err != nil {
		http.Error(w, `{"error":"sprint not found"}`, http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(sp)
}

func (h *SprintHandler) Complete(w http.ResponseWriter, r *http.Request) {
	var req struct {
		MoveOpenToBacklog bool `json:"move_open_to_backlog"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	sp, err := h.svc.Complete(r.PathValue("id"), req.MoveOpenToBacklog)
	if err != nil {
		http.Error(w, `{"error":"sprint not found"}`, http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(sp)
}
