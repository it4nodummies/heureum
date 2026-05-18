package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/open-jira/open-jira/internal/api/middleware"
	"github.com/open-jira/open-jira/internal/domain/issue"
)

type CommentHandler struct {
	svc      *issue.CommentService
	issueSvc *issue.Service
}

func NewCommentHandler(svc *issue.CommentService, issueSvc *issue.Service) *CommentHandler {
	return &CommentHandler{svc: svc, issueSvc: issueSvc}
}

func (h *CommentHandler) Get(w http.ResponseWriter, r *http.Request) {
	comment, err := h.svc.GetComment(r.PathValue("id"))
	if err != nil {
		http.Error(w, `{"error":"comment not found"}`, http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(comment)
}

func (h *CommentHandler) List(w http.ResponseWriter, r *http.Request) {
	iss, err := h.issueSvc.GetByKey(r.PathValue("issueIdOrKey"))
	if err != nil {
		http.Error(w, `{"error":"issue not found"}`, http.StatusNotFound)
		return
	}
	comments, _ := h.svc.GetComments(iss.ID)
	startAt, maxResults := parsePagination(r)
	total := len(comments)
	paged := paginateSlice(comments, startAt, maxResults)
	resp := pagedResponse{
		StartAt: startAt, MaxResults: maxResults, Total: total,
		IsLast: startAt+len(paged) >= total, Values: paged,
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (h *CommentHandler) Create(w http.ResponseWriter, r *http.Request) {
	iss, err := h.issueSvc.GetByKey(r.PathValue("issueIdOrKey"))
	if err != nil {
		http.Error(w, `{"error":"issue not found"}`, http.StatusNotFound)
		return
	}
	var req struct {
		BodyJSON string `json:"body_json"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}
	if req.BodyJSON == "" {
		http.Error(w, `{"error":"body_json is required"}`, http.StatusBadRequest)
		return
	}
	authorID := middleware.UserIDFromContext(r.Context())
	comment, err := h.svc.AddComment(iss.ID, authorID, req.BodyJSON)
	if err != nil {
		http.Error(w, `{"error":"failed to add comment"}`, http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(comment)
}

func (h *CommentHandler) Update(w http.ResponseWriter, r *http.Request) {
	var req struct {
		BodyJSON string `json:"body_json"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}
	if req.BodyJSON == "" {
		http.Error(w, `{"error":"body_json is required"}`, http.StatusBadRequest)
		return
	}
	comment, err := h.svc.UpdateComment(r.PathValue("id"), req.BodyJSON)
	if err != nil {
		http.Error(w, `{"error":"comment not found"}`, http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(comment)
}

func (h *CommentHandler) Delete(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.SoftDeleteComment(r.PathValue("id")); err != nil {
		http.Error(w, `{"error":"failed to delete comment"}`, http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *CommentHandler) ListByIDs(w http.ResponseWriter, r *http.Request) {
	var req struct {
		IDs []string `json:"ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}
	if len(req.IDs) == 0 {
		http.Error(w, `{"error":"ids is required"}`, http.StatusBadRequest)
		return
	}
	comments, err := h.svc.GetCommentsByIDs(req.IDs)
	if err != nil {
		http.Error(w, `{"error":"failed to get comments"}`, http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(comments)
}

func parsePagination(r *http.Request) (int, int) {
	startAt := 0
	maxResults := 50
	if v := r.URL.Query().Get("startAt"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			startAt = n
		}
	}
	if v := r.URL.Query().Get("maxResults"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			maxResults = n
		}
	}
	return startAt, maxResults
}

type pagedResponse struct {
	StartAt    int         `json:"startAt"`
	MaxResults int         `json:"maxResults"`
	Total      int         `json:"total"`
	IsLast     bool        `json:"isLast"`
	Values     interface{} `json:"values"`
}

func paginateSlice[T any](items []T, startAt, maxResults int) []T {
	if startAt >= len(items) {
		return []T{}
	}
	end := startAt + maxResults
	if end > len(items) {
		end = len(items)
	}
	return items[startAt:end]
}
