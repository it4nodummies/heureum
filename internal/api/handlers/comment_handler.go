package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/it4nodummies/heureum/internal/api/middleware"
	v3 "github.com/it4nodummies/heureum/internal/api/v3"
	"github.com/it4nodummies/heureum/internal/domain/issue"
	"github.com/it4nodummies/heureum/internal/domain/user"
)

// toComment costruisce la v3.Comment di risposta. I timestamp conformi al
// contratto sono prodotti da v3.JiraTime (offset RFC3339 con ":").
func (h *CommentHandler) toComment(c issue.Comment, author *user.User) v3.Comment {
	return v3.JiraComment(c, author, author, h.baseURL)
}

type CommentHandler struct {
	svc      *issue.CommentService
	issueSvc *issue.Service
	baseURL  string
}

func NewCommentHandler(svc *issue.CommentService, issueSvc *issue.Service, baseURL string) *CommentHandler {
	return &CommentHandler{svc: svc, issueSvc: issueSvc, baseURL: baseURL}
}

// resolve trova l'issue dal path param issueIdOrKey, provando prima come ID
// numerico sequenziale e poi come key (es. "DEMO-1").
func (h *CommentHandler) resolve(r *http.Request) *issue.Issue {
	k := r.PathValue("issueIdOrKey")
	if n, err := strconv.ParseInt(k, 10, 64); err == nil {
		if iss, err := h.issueSvc.GetBySeqID(n); err == nil {
			return iss
		}
		return nil
	}
	iss, err := h.issueSvc.GetByKey(k)
	if err != nil {
		return nil
	}
	return iss
}

// author carica lo user autore di un commento, se presente.
func (h *CommentHandler) author(c *issue.Comment) *user.User {
	if c.AuthorID == nil {
		return nil
	}
	var u user.User
	if h.issueSvc.DB().First(&u, "id = ?", *c.AuthorID).Error != nil {
		return nil
	}
	return &u
}

func (h *CommentHandler) Get(w http.ResponseWriter, r *http.Request) {
	comment, err := h.svc.GetComment(r.PathValue("id"))
	if err != nil {
		v3.WriteError(w, http.StatusNotFound, []string{"comment not found"}, nil)
		return
	}
	author := h.author(comment)
	v3.WriteJSON(w, http.StatusOK, h.toComment(*comment, author))
}

func (h *CommentHandler) List(w http.ResponseWriter, r *http.Request) {
	iss := h.resolve(r)
	if iss == nil {
		v3.WriteError(w, http.StatusNotFound, []string{"issue not found"}, nil)
		return
	}
	comments, err := h.svc.GetComments(iss.ID)
	if err != nil {
		v3.WriteError(w, http.StatusInternalServerError, []string{"failed to get comments"}, nil)
		return
	}
	total := len(comments)
	// Default e cap a 100, come la GET comments di Jira Cloud.
	startAt, maxResults := v3.ParsePagination(r, 100, 100)
	lo := startAt
	if lo > total {
		lo = total
	}
	hi := lo + maxResults
	if hi > total {
		hi = total
	}
	window := comments[lo:hi]
	out := make([]v3.Comment, 0, len(window))
	for _, c := range window {
		author := h.author(&c)
		out = append(out, h.toComment(c, author))
	}
	v3.WriteJSON(w, http.StatusOK, v3.PageOfComments{
		StartAt:    startAt,
		MaxResults: maxResults,
		Total:      total,
		Comments:   out,
	})
}

func (h *CommentHandler) Create(w http.ResponseWriter, r *http.Request) {
	iss := h.resolve(r)
	if iss == nil {
		v3.WriteError(w, http.StatusNotFound, []string{"issue not found"}, nil)
		return
	}
	var req struct {
		Body any `json:"body"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		v3.WriteError(w, http.StatusBadRequest, []string{"invalid request body"}, nil)
		return
	}
	if req.Body == nil {
		v3.WriteError(w, http.StatusBadRequest, []string{"body is required"}, nil)
		return
	}
	bodyJSON, err := json.Marshal(req.Body)
	if err != nil {
		v3.WriteError(w, http.StatusBadRequest, []string{"invalid body"}, nil)
		return
	}
	authorID := middleware.UserIDFromContext(r.Context())
	// Estrae gli accountId dai nodi ADF mention (attrs.id = user id) e li passa
	// al service, che li unisce alle mention testuali @username notificando ogni
	// utente una sola volta (dedupe lato notifier).
	mentionUserIDs := v3.ExtractMentions(string(bodyJSON))
	comment, err := h.svc.AddComment(iss.ID, authorID, string(bodyJSON), mentionUserIDs...)
	if err != nil {
		v3.WriteError(w, http.StatusInternalServerError, []string{"failed to add comment"}, nil)
		return
	}
	author := h.author(comment)
	v3.WriteJSON(w, http.StatusCreated, h.toComment(*comment, author))
}

func (h *CommentHandler) Update(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Body any `json:"body"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		v3.WriteError(w, http.StatusBadRequest, []string{"invalid request body"}, nil)
		return
	}
	if req.Body == nil {
		v3.WriteError(w, http.StatusBadRequest, []string{"body is required"}, nil)
		return
	}
	bodyJSON, err := json.Marshal(req.Body)
	if err != nil {
		v3.WriteError(w, http.StatusBadRequest, []string{"invalid body"}, nil)
		return
	}
	comment, err := h.svc.UpdateComment(r.PathValue("id"), string(bodyJSON))
	if err != nil {
		v3.WriteError(w, http.StatusNotFound, []string{"comment not found"}, nil)
		return
	}
	author := h.author(comment)
	v3.WriteJSON(w, http.StatusOK, h.toComment(*comment, author))
}

func (h *CommentHandler) Delete(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.SoftDeleteComment(r.PathValue("id")); err != nil {
		v3.WriteError(w, http.StatusInternalServerError, []string{"failed to delete comment"}, nil)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ListByIDs implementa POST /rest/api/3/comment/list, che fa parte del
// contratto Jira v3 (schema IssueCommentListRequestBean). Deviazione nota:
// il nostro request body usa ID stringa ("ids") anziché gli ID numerici
// dello schema ufficiale; la risposta è comunque una PageOfComments.
func (h *CommentHandler) ListByIDs(w http.ResponseWriter, r *http.Request) {
	var req struct {
		IDs []string `json:"ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		v3.WriteError(w, http.StatusBadRequest, []string{"invalid request body"}, nil)
		return
	}
	if len(req.IDs) == 0 {
		v3.WriteError(w, http.StatusBadRequest, []string{"ids is required"}, nil)
		return
	}
	comments, err := h.svc.GetCommentsByIDs(req.IDs)
	if err != nil {
		v3.WriteError(w, http.StatusInternalServerError, []string{"failed to get comments"}, nil)
		return
	}
	out := make([]v3.Comment, 0, len(comments))
	for _, c := range comments {
		author := h.author(&c)
		out = append(out, h.toComment(c, author))
	}
	v3.WriteJSON(w, http.StatusOK, v3.PageOfComments{
		StartAt:    0,
		MaxResults: len(out),
		Total:      len(out),
		Comments:   out,
	})
}
