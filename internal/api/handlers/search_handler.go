package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/open-jira/open-jira/internal/domain/issue"
	"github.com/open-jira/open-jira/internal/domain/search"
)

type SearchHandler struct {
	svc *search.Service
}

func NewSearchHandler(svc *search.Service) *SearchHandler {
	return &SearchHandler{svc: svc}
}

func (h *SearchHandler) Search(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	issues, err := h.svc.Search(query)
	if err != nil {
		http.Error(w, `{"error":"search failed"}`, http.StatusInternalServerError)
		return
	}
	if issues == nil {
		issues = []issue.Issue{}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(issues)
}
