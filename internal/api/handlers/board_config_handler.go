package handlers

import (
	"encoding/json"
	"net/http"
	"sort"

	v3 "github.com/it4nodummies/heureum/internal/api/v3"
	"github.com/it4nodummies/heureum/internal/domain/board"
)

// fallbackStatuses costruisce l'elenco di stati 1:1 (id+name, ordinati per
// position) usato come default quando la board non ha colonne persistite.
func (h *AgileBoardHandler) fallbackStatuses(projectID string) []board.FallbackStatus {
	wf, err := h.workflowSvc.GetWorkflow(projectID)
	if err != nil {
		return nil
	}
	sort.Slice(wf.Statuses, func(i, j int) bool { return wf.Statuses[i].Position < wf.Statuses[j].Position })
	out := make([]board.FallbackStatus, 0, len(wf.Statuses))
	for _, st := range wf.Statuses {
		out = append(out, board.FallbackStatus{ID: st.ID, Name: st.Name})
	}
	return out
}

// GetCustomConfig è l'endpoint Heureum-custom (NON conforme Jira) che espone la
// configurazione completa e editabile della board: colonne (con i loro status
// id), swimlane mode e quick filters. GET /rest/agile/1.0/board/{boardId}/config
func (h *AgileBoardHandler) GetCustomConfig(w http.ResponseWriter, r *http.Request) {
	b := h.resolveBoard(r)
	if b == nil {
		v3.WriteError(w, http.StatusNotFound, []string{"board not found"}, nil)
		return
	}
	cfg, err := h.boardSvc.GetConfig(b.ID, h.fallbackStatuses(b.ProjectID))
	if err != nil {
		v3.WriteError(w, http.StatusInternalServerError, []string{"failed to load board config"}, nil)
		return
	}
	v3.WriteJSON(w, http.StatusOK, cfg)
}

// SaveCustomConfig sostituisce la configurazione della board (colonne + status
// set, swimlane, quick filters) e restituisce la config salvata.
// PUT /rest/agile/1.0/board/{boardId}/config
func (h *AgileBoardHandler) SaveCustomConfig(w http.ResponseWriter, r *http.Request) {
	b := h.resolveBoard(r)
	if b == nil {
		v3.WriteError(w, http.StatusNotFound, []string{"board not found"}, nil)
		return
	}
	var in board.BoardConfigInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		v3.WriteError(w, http.StatusBadRequest, []string{"invalid request body"}, nil)
		return
	}
	if err := h.boardSvc.SaveConfig(b.ID, in); err != nil {
		v3.WriteError(w, http.StatusInternalServerError, []string{"failed to save board config"}, nil)
		return
	}
	cfg, err := h.boardSvc.GetConfig(b.ID, h.fallbackStatuses(b.ProjectID))
	if err != nil {
		v3.WriteError(w, http.StatusInternalServerError, []string{"failed to load board config"}, nil)
		return
	}
	v3.WriteJSON(w, http.StatusOK, cfg)
}
