package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/it4nodummies/heureum/internal/api/middleware"
	v3 "github.com/it4nodummies/heureum/internal/api/v3"
	"github.com/it4nodummies/heureum/internal/domain/issue"
	"github.com/it4nodummies/heureum/internal/domain/user"
)

// WorklogHandler serve gli endpoint Jira v3 di time tracking (worklog) su un issue.
type WorklogHandler struct {
	svc      *issue.WorklogService
	issueSvc *issue.Service
	baseURL  string
}

func NewWorklogHandler(svc *issue.WorklogService, issueSvc *issue.Service, baseURL string) *WorklogHandler {
	return &WorklogHandler{svc: svc, issueSvc: issueSvc, baseURL: baseURL}
}

// resolve trova l'issue dal path param issueIdOrKey, provando prima come ID
// numerico sequenziale e poi come key (es. "DEMO-1").
func (h *WorklogHandler) resolve(r *http.Request) *issue.Issue {
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

// author carica lo user autore di un worklog, se presente.
func (h *WorklogHandler) author(authorID *string) *user.User {
	if authorID == nil {
		return nil
	}
	var u user.User
	if h.issueSvc.DB().First(&u, "id = ?", *authorID).Error != nil {
		return nil
	}
	return &u
}

func (h *WorklogHandler) toWorklog(wl issue.Worklog) v3.Worklog {
	return v3.JiraWorklog(wl, h.author(wl.AuthorID), h.baseURL)
}

// List restituisce i worklog di un issue come PageOfWorklogs.
func (h *WorklogHandler) List(w http.ResponseWriter, r *http.Request) {
	iss := h.resolve(r)
	if iss == nil {
		v3.WriteError(w, http.StatusNotFound, []string{"issue not found"}, nil)
		return
	}
	worklogs, err := h.svc.ListByIssue(iss.ID)
	if err != nil {
		v3.WriteError(w, http.StatusInternalServerError, []string{"failed to get worklogs"}, nil)
		return
	}
	out := make([]v3.Worklog, 0, len(worklogs))
	for _, wl := range worklogs {
		out = append(out, h.toWorklog(wl))
	}
	v3.WriteJSON(w, http.StatusOK, v3.PageOfWorklogs{
		StartAt:    0,
		MaxResults: len(out),
		Total:      len(out),
		Worklogs:   out,
	})
}

// createWorklogRequest è il body accettato da POST .../worklog: timeSpentSeconds
// o timeSpent (mutuamente alternativi, come nel contratto), comment ADF opzionale.
type createWorklogRequest struct {
	Comment          any    `json:"comment"`
	TimeSpent        string `json:"timeSpent"`
	TimeSpentSeconds int    `json:"timeSpentSeconds"`
}

// parseJiraDuration converte una stringa timeSpent in stile Jira ("1d 2h 30m",
// "3h", "45m", "2w") in secondi. Le unità supportate sono w(eek)=5*8h, d(ay)=8h,
// h(our)=3600s, m(inute)=60s; token senza suffisso sono trattati come minuti.
func parseJiraDuration(s string) int {
	total := 0
	for _, tok := range strings.Fields(s) {
		if tok == "" {
			continue
		}
		unit := tok[len(tok)-1:]
		numStr := tok
		switch unit {
		case "w", "d", "h", "m":
			numStr = tok[:len(tok)-1]
		default:
			unit = "m"
		}
		n, err := strconv.Atoi(numStr)
		if err != nil {
			continue
		}
		switch unit {
		case "w":
			total += n * 5 * 8 * 3600
		case "d":
			total += n * 8 * 3600
		case "h":
			total += n * 3600
		case "m":
			total += n * 60
		}
	}
	return total
}

// Create registra un nuovo worklog sull'issue risolto dal path.
func (h *WorklogHandler) Create(w http.ResponseWriter, r *http.Request) {
	iss := h.resolve(r)
	if iss == nil {
		v3.WriteError(w, http.StatusNotFound, []string{"issue not found"}, nil)
		return
	}
	var req createWorklogRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		v3.WriteError(w, http.StatusBadRequest, []string{"invalid request body"}, nil)
		return
	}
	seconds := req.TimeSpentSeconds
	if seconds == 0 && req.TimeSpent != "" {
		seconds = parseJiraDuration(req.TimeSpent)
	}
	if seconds == 0 {
		v3.WriteError(w, http.StatusBadRequest, []string{"timeSpent or timeSpentSeconds is required"}, nil)
		return
	}
	commentJSON := ""
	if req.Comment != nil {
		b, err := json.Marshal(req.Comment)
		if err != nil {
			v3.WriteError(w, http.StatusBadRequest, []string{"invalid comment"}, nil)
			return
		}
		commentJSON = string(b)
	}
	authorID := middleware.UserIDFromContext(r.Context())
	wl, err := h.svc.Add(iss.ID, authorID, commentJSON, seconds)
	if err != nil {
		v3.WriteError(w, http.StatusInternalServerError, []string{"failed to add worklog"}, nil)
		return
	}
	v3.WriteJSON(w, http.StatusCreated, h.toWorklog(*wl))
}

// Delete rimuove il worklog identificato dal path param "id".
func (h *WorklogHandler) Delete(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.Delete(r.PathValue("id")); err != nil {
		v3.WriteError(w, http.StatusInternalServerError, []string{"failed to delete worklog"}, nil)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
