package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/open-jira/open-jira/internal/api/middleware"
	"github.com/open-jira/open-jira/internal/domain/issue"
	"github.com/open-jira/open-jira/internal/domain/search"
)

type SearchHandler struct {
	svc     *search.Service
	filters *search.FilterService
}

func NewSearchHandler(svc *search.Service, filters *search.FilterService) *SearchHandler {
	return &SearchHandler{svc: svc, filters: filters}
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

func (h *SearchHandler) SearchPost(w http.ResponseWriter, r *http.Request) {
	var req struct {
		JQL        string `json:"jql"`
		StartAt    int    `json:"startAt"`
		MaxResults int    `json:"maxResults"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}
	issues, err := h.svc.Search(req.JQL)
	if err != nil {
		http.Error(w, `{"error":"search failed"}`, http.StatusInternalServerError)
		return
	}
	if issues == nil {
		issues = []issue.Issue{}
	}
	if req.MaxResults <= 0 {
		req.MaxResults = 50
	}
	if req.StartAt > len(issues) {
		req.StartAt = len(issues)
	}
	end := req.StartAt + req.MaxResults
	if end > len(issues) {
		end = len(issues)
	}
	paginated := issues[req.StartAt:end]

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"issues":     paginated,
		"total":      len(issues),
		"startAt":    req.StartAt,
		"maxResults": req.MaxResults,
	})
}

func (h *SearchHandler) ListMyFilters(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	filters, err := h.filters.List(userID)
	if err != nil {
		http.Error(w, `{"error":"failed to list filters"}`, http.StatusInternalServerError)
		return
	}
	if filters == nil {
		filters = []search.SavedFilter{}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(filters)
}

func (h *SearchHandler) ListFavouriteFilters(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	filters, err := h.filters.ListFavourites(userID)
	if err != nil {
		http.Error(w, `{"error":"failed to list filters"}`, http.StatusInternalServerError)
		return
	}
	if filters == nil {
		filters = []search.SavedFilter{}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(filters)
}

func (h *SearchHandler) UpdateFilter(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name     string `json:"name"`
		JQL      string `json:"jql"`
		IsShared *bool  `json:"is_shared"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}
	f, err := h.filters.Update(r.PathValue("id"), req.Name, req.JQL, req.IsShared)
	if err != nil {
		http.Error(w, `{"error":"filter not found"}`, http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(f)
}

func (h *SearchHandler) AddFavourite(w http.ResponseWriter, r *http.Request) {
	if err := h.filters.ToggleFavourite(r.PathValue("id"), true); err != nil {
		http.Error(w, `{"error":"filter not found"}`, http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *SearchHandler) RemoveFavourite(w http.ResponseWriter, r *http.Request) {
	if err := h.filters.ToggleFavourite(r.PathValue("id"), false); err != nil {
		http.Error(w, `{"error":"filter not found"}`, http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *SearchHandler) ChangeFilterOwner(w http.ResponseWriter, r *http.Request) {
	var req struct {
		AccountID string `json:"accountId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}
	if err := h.filters.ChangeOwner(r.PathValue("id"), req.AccountID); err != nil {
		http.Error(w, `{"error":"filter not found"}`, http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *SearchHandler) CreateFilter(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	var req struct {
		Name     string  `json:"name"`
		JQL      string  `json:"jql"`
		IsShared bool    `json:"is_shared"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}
	f, err := h.filters.Create(userID, nil, req.Name, req.JQL, req.IsShared)
	if err != nil {
		http.Error(w, `{"error":"failed to create filter"}`, http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(f)
}

func (h *SearchHandler) GetFilter(w http.ResponseWriter, r *http.Request) {
	f, err := h.filters.Get(r.PathValue("id"))
	if err != nil {
		http.Error(w, `{"error":"filter not found"}`, http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(f)
}

func (h *SearchHandler) DeleteFilter(w http.ResponseWriter, r *http.Request) {
	if err := h.filters.Delete(r.PathValue("id")); err != nil {
		http.Error(w, `{"error":"filter not found"}`, http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
