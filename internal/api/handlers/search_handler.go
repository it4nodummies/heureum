package handlers

import (
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/it4nodummies/heureum/internal/api/middleware"
	v3 "github.com/it4nodummies/heureum/internal/api/v3"
	"github.com/it4nodummies/heureum/internal/domain/issue"
	"github.com/it4nodummies/heureum/internal/domain/search"
)

// SearchHandler implementa gli endpoint di ricerca JQL conformi a Jira Cloud
// REST API v3: /search/jql (token-paginato), /search (legacy, offset-paginato)
// e /search/approximate-count.
type SearchHandler struct {
	svc    *search.Service
	issueH *IssueHandler // riusa buildIssueInput per costruire gli IssueBean
}

func NewSearchHandler(svc *search.Service, issueH *IssueHandler) *SearchHandler {
	return &SearchHandler{svc: svc, issueH: issueH}
}

// jqlBody è il corpo condiviso da tutti gli endpoint di ricerca, letto sia dal
// body JSON (POST) sia dalla query string (GET).
type jqlBody struct {
	JQL           string   `json:"jql"`
	MaxResults    int      `json:"maxResults"`
	NextPageToken string   `json:"nextPageToken"`
	StartAt       int      `json:"startAt"`
	Fields        []string `json:"fields"`
}

// readParams estrae jql/fields/maxResults/nextPageToken/startAt dal POST
// (body JSON) o dal GET (query string).
func (h *SearchHandler) readParams(r *http.Request) (jqlBody, error) {
	var b jqlBody
	if r.Method == http.MethodPost {
		if err := json.NewDecoder(r.Body).Decode(&b); err != nil && err != io.EOF {
			return b, err
		}
		return b, nil
	}
	q := r.URL.Query()
	b.JQL = q.Get("jql")
	b.NextPageToken = q.Get("nextPageToken")
	if v := q.Get("fields"); v != "" {
		b.Fields = splitCSV(v)
	}
	if v := q.Get("maxResults"); v != "" {
		if n, err := parseIntSafe(v); err == nil {
			b.MaxResults = n
		}
	}
	if v := q.Get("startAt"); v != "" {
		if n, err := parseIntSafe(v); err == nil {
			b.StartAt = n
		}
	}
	return b, nil
}

// renderIssues costruisce gli IssueBean con proiezione dei fields richiesti.
func (h *SearchHandler) renderIssues(issues []issue.Issue, fields []string) ([]map[string]any, error) {
	return renderIssueList(h.issueH, issues, fields)
}

// SearchJQL gestisce GET/POST /rest/api/3/search/jql (token-paginato).
func (h *SearchHandler) SearchJQL(w http.ResponseWriter, r *http.Request) {
	b, err := h.readParams(r)
	if err != nil {
		v3.WriteError(w, http.StatusBadRequest, []string{"invalid request body"}, nil)
		return
	}
	offset, err := v3.DecodeCursor(b.NextPageToken)
	if err != nil {
		v3.WriteError(w, http.StatusBadRequest, []string{"invalid nextPageToken"}, nil)
		return
	}
	limit := clampMax(b.MaxResults, 50, 100)
	uid := middleware.UserIDFromContext(r.Context())
	res, err := h.svc.Search(b.JQL, search.NewDBResolver(h.svc.DB(), uid), offset, limit)
	if err != nil {
		v3.WriteError(w, http.StatusBadRequest, []string{"invalid JQL: " + err.Error()}, nil)
		return
	}
	items, err := h.renderIssues(res.Issues, b.Fields)
	if err != nil {
		v3.WriteError(w, http.StatusInternalServerError, []string{"render error"}, nil)
		return
	}
	isLast := offset+len(res.Issues) >= res.Total
	out := v3.SearchAndReconcileResults{Issues: items, IsLast: isLast}
	if !isLast {
		out.NextPageToken = v3.EncodeCursor(offset + limit)
	}
	v3.WriteJSON(w, http.StatusOK, out)
}

// SearchLegacy gestisce GET/POST /rest/api/3/search (offset-paginato).
func (h *SearchHandler) SearchLegacy(w http.ResponseWriter, r *http.Request) {
	b, err := h.readParams(r)
	if err != nil {
		v3.WriteError(w, http.StatusBadRequest, []string{"invalid request body"}, nil)
		return
	}
	if b.StartAt < 0 {
		b.StartAt = 0
	}
	limit := clampMax(b.MaxResults, 50, 100)
	uid := middleware.UserIDFromContext(r.Context())
	res, err := h.svc.Search(b.JQL, search.NewDBResolver(h.svc.DB(), uid), b.StartAt, limit)
	if err != nil {
		v3.WriteError(w, http.StatusBadRequest, []string{"invalid JQL: " + err.Error()}, nil)
		return
	}
	items, err := h.renderIssues(res.Issues, b.Fields)
	if err != nil {
		v3.WriteError(w, http.StatusInternalServerError, []string{"render error"}, nil)
		return
	}
	v3.WriteJSON(w, http.StatusOK, v3.SearchResults{
		Issues: items, StartAt: b.StartAt, MaxResults: limit, Total: res.Total,
	})
}

// ApproximateCount gestisce POST /rest/api/3/search/approximate-count.
func (h *SearchHandler) ApproximateCount(w http.ResponseWriter, r *http.Request) {
	b, err := h.readParams(r)
	if err != nil {
		v3.WriteError(w, http.StatusBadRequest, []string{"invalid request body"}, nil)
		return
	}
	uid := middleware.UserIDFromContext(r.Context())
	res, err := h.svc.Search(b.JQL, search.NewDBResolver(h.svc.DB(), uid), 0, 0)
	if err != nil {
		v3.WriteError(w, http.StatusBadRequest, []string{"invalid JQL: " + err.Error()}, nil)
		return
	}
	v3.WriteJSON(w, http.StatusOK, map[string]any{"count": res.Total})
}

// splitCSV divide una lista comma-separated scartando i token vuoti.
func splitCSV(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// parseIntSafe è un wrapper su strconv.Atoi usato per i query param numerici.
func parseIntSafe(s string) (int, error) {
	return strconv.Atoi(s)
}

// clampMax applica il default/cap in stile Jira: v<=0 => def, v>cap => cap.
func clampMax(v, def, cap int) int {
	if v <= 0 {
		return def
	}
	if v > cap {
		return cap
	}
	return v
}

// Autocomplete gestisce GET /rest/api/3/jql/autocompletedata.
func (h *SearchHandler) Autocomplete(w http.ResponseWriter, r *http.Request) {
	v3.WriteJSON(w, http.StatusOK, v3.AutocompleteData())
}
