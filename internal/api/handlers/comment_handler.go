package handlers

import (
	"encoding/json"
	"net/http"

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

func (h *CommentHandler) List(w http.ResponseWriter, r *http.Request) {
	iss, err := h.issueSvc.GetByKey(r.PathValue("issueKey"))
	if err != nil {
		http.Error(w, `{"error":"issue not found"}`, http.StatusNotFound)
		return
	}
	comments, _ := h.svc.GetComments(iss.ID)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(comments)
}

func (h *CommentHandler) Create(w http.ResponseWriter, r *http.Request) {
	iss, err := h.issueSvc.GetByKey(r.PathValue("issueKey"))
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

func (h *CommentHandler) Delete(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.SoftDeleteComment(r.PathValue("commentId")); err != nil {
		http.Error(w, `{"error":"failed to delete comment"}`, http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
