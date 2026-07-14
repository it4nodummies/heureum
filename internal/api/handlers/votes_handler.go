package handlers

import (
	"net/http"
	"strconv"

	"github.com/it4nodummies/heureum/internal/api/middleware"
	v3 "github.com/it4nodummies/heureum/internal/api/v3"
	"github.com/it4nodummies/heureum/internal/domain/issue"
	"github.com/it4nodummies/heureum/internal/domain/user"
)

// VotesHandler serve gli endpoint Jira v3 dei voti su un issue.
type VotesHandler struct {
	svc      *issue.VoteService
	issueSvc *issue.Service
	baseURL  string
}

func NewVotesHandler(svc *issue.VoteService, issueSvc *issue.Service, baseURL string) *VotesHandler {
	return &VotesHandler{svc: svc, issueSvc: issueSvc, baseURL: baseURL}
}

// resolve trova l'issue dal path param issueIdOrKey, provando prima come ID
// numerico sequenziale e poi come key (es. "DEMO-1").
func (h *VotesHandler) resolve(r *http.Request) *issue.Issue {
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

// voters carica gli user.User che hanno votato issueID.
func (h *VotesHandler) voters(issueID string) []user.User {
	ids := h.svc.Voters(issueID)
	if len(ids) == 0 {
		return nil
	}
	var out []user.User
	h.issueSvc.DB().Find(&out, "id IN ?", ids)
	return out
}

// List restituisce lo stato dei voti dell'issue come Votes.
func (h *VotesHandler) List(w http.ResponseWriter, r *http.Request) {
	iss := h.resolve(r)
	if iss == nil {
		v3.WriteError(w, http.StatusNotFound, []string{"issue not found"}, nil)
		return
	}
	userID := middleware.UserIDFromContext(r.Context())
	v3.WriteJSON(w, http.StatusOK, v3.JiraVotes(
		iss.Key,
		h.baseURL,
		h.svc.Count(iss.ID),
		h.svc.HasVoted(iss.ID, userID),
		h.voters(iss.ID),
	))
}

// Add registra il voto dell'utente corrente sull'issue.
func (h *VotesHandler) Add(w http.ResponseWriter, r *http.Request) {
	iss := h.resolve(r)
	if iss == nil {
		v3.WriteError(w, http.StatusNotFound, []string{"issue not found"}, nil)
		return
	}
	userID := middleware.UserIDFromContext(r.Context())
	if err := h.svc.Add(iss.ID, userID); err != nil {
		v3.WriteError(w, http.StatusInternalServerError, []string{"failed to add vote"}, nil)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// Remove rimuove il voto dell'utente corrente sull'issue.
func (h *VotesHandler) Remove(w http.ResponseWriter, r *http.Request) {
	iss := h.resolve(r)
	if iss == nil {
		v3.WriteError(w, http.StatusNotFound, []string{"issue not found"}, nil)
		return
	}
	userID := middleware.UserIDFromContext(r.Context())
	if err := h.svc.Remove(iss.ID, userID); err != nil {
		v3.WriteError(w, http.StatusInternalServerError, []string{"failed to remove vote"}, nil)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
