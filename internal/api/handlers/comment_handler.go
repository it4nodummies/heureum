package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/open-jira/open-jira/internal/api/middleware"
	v3 "github.com/open-jira/open-jira/internal/api/v3"
	"github.com/open-jira/open-jira/internal/domain/issue"
	"github.com/open-jira/open-jira/internal/domain/user"
)

// rfc3339 formatta i timestamp con offset separato da ":" (es. "+00:00"),
// come richiesto dal pattern "date-time" del contratto Jira v3 per Comment
// (a differenza di v3.JiraTime, usato altrove, che produce "+0000" senza
// ":" e non è validato in modo stretto sugli altri schemi).
func rfc3339(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format("2006-01-02T15:04:05.000-07:00")
}

// toComment costruisce la v3.Comment di risposta correggendo i timestamp
// prodotti da v3.JiraComment nel formato richiesto dal contratto.
func (h *CommentHandler) toComment(c issue.Comment, author *user.User) v3.Comment {
	jc := v3.JiraComment(c, author, author, h.baseURL)
	jc.Created = rfc3339(c.CreatedAt)
	jc.Updated = rfc3339(c.UpdatedAt)
	return jc
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
	comment, err := h.svc.AddComment(iss.ID, authorID, string(bodyJSON))
	if err != nil {
		v3.WriteError(w, http.StatusInternalServerError, []string{"failed to add comment"}, nil)
		return
	}
	// Le mention vengono estratte per un eventuale utilizzo futuro (es.
	// notifiche mirate su ADF); la notifica "commented" e quella basata su
	// @username testuale sono già gestite da CommentService.AddComment.
	_ = v3.ExtractMentions(string(bodyJSON))
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

// ListByIDs è un'estensione non-standard (POST /comment/list) usata dal
// frontend per risolvere in batch i commenti citati altrove; non fa parte
// del contratto Jira v3, quindi non emette la forma v3.Comment.
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
