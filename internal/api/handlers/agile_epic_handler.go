package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"sort"
	"strings"

	"gorm.io/gorm"

	"github.com/it4nodummies/heureum/internal/api/authz"
	"github.com/it4nodummies/heureum/internal/api/middleware"
	v3 "github.com/it4nodummies/heureum/internal/api/v3"
	"github.com/it4nodummies/heureum/internal/domain/issue"
	"github.com/it4nodummies/heureum/internal/domain/permission"
	"github.com/it4nodummies/heureum/internal/domain/search"
	"github.com/it4nodummies/heureum/internal/domain/workflow"
	"github.com/it4nodummies/heureum/internal/jql"
)

// Agile Epic API (Round 28): the /rest/agile/1.0/epic/* routes of the
// jira-agile-1.0 spec plus the board-scoped epic issue lists. An epic is an
// issue whose type sits at hierarchy level 1; its `done` is computed from the
// status category (no second source of truth) and its colour is
// issues.epic_color (one of issue.EpicColorKeys).

// epicBean is the epic shape promised by GET/POST /epic/{id} and
// GET /board/{id}/epic: {id, key, self, name, summary, color{key}, done}.
// Heureum keeps one title per issue, so name and summary are both the title.
// An epic without a colour (never happens after the backfill) renders the
// default key rather than null.
func epicBean(baseURL string, iss *issue.Issue, done bool) map[string]any {
	color := issue.DefaultEpicColorKey
	if iss.EpicColor != nil && issue.IsEpicColorKey(*iss.EpicColor) {
		color = *iss.EpicColor
	}
	return map[string]any{
		"id":      iss.SeqID,
		"key":     iss.Key,
		"self":    baseURL + "/rest/agile/1.0/epic/" + iss.Key,
		"name":    iss.Title,
		"summary": iss.Title,
		"color":   map[string]any{"key": color},
		"done":    done,
	}
}

// issueLevel returns the hierarchy level of the issue's type (0 for an issue
// without a type).
func issueLevel(db *gorm.DB, iss *issue.Issue) int {
	if iss.TypeID == nil {
		return 0
	}
	var it issue.IssueType
	if db.Select("hierarchy_level").First(&it, "id = ?", *iss.TypeID).Error != nil {
		return 0
	}
	return it.HierarchyLevel
}

// resolveEpicRef resolves idOrKey to an issue and returns it only when it is
// an epic (level 1): a story or an unknown key both resolve to nil, so the
// routes answer 404 for either.
func resolveEpicRef(issueSvc *issue.Service, idOrKey string) *issue.Issue {
	iss := resolveIssueRef(issueSvc, idOrKey)
	if iss == nil || issueLevel(issueSvc.DB(), iss) != 1 {
		return nil
	}
	return iss
}

func (h *AgileMiscHandler) resolveEpic(idOrKey string) *issue.Issue {
	return resolveEpicRef(h.issueSvc, idOrKey)
}

// doneStatusSet returns, among the statuses of the given issues, the ids
// whose category is done — one query for a whole list (no N+1).
func doneStatusSet(db *gorm.DB, issues []issue.Issue) map[string]bool {
	seen := map[string]bool{}
	ids := make([]string, 0, len(issues))
	for i := range issues {
		if sid := issues[i].StatusID; sid != nil && !seen[*sid] {
			seen[*sid] = true
			ids = append(ids, *sid)
		}
	}
	out := map[string]bool{}
	if len(ids) == 0 {
		return out
	}
	var doneIDs []string
	if err := db.Model(&workflow.WorkflowStatus{}).Where("category = ? AND id IN (?)", workflow.CategoryDone, ids).Pluck("id", &doneIDs).Error; err != nil {
		return out
	}
	for _, id := range doneIDs {
		out[id] = true
	}
	return out
}

// isDoneStatus reports whether statusID (nil = no status) is a done-category status.
func isDoneStatus(db *gorm.DB, statusID *string) bool {
	if statusID == nil {
		return false
	}
	var n int64
	db.Model(&workflow.WorkflowStatus{}).Where("id = ? AND category = ?", *statusID, workflow.CategoryDone).Count(&n)
	return n > 0
}

// writeEpicIssuePage lists one page of issues matching opts (plus the
// optional `jql` query parameter compiled with the caller's resolver, 400 when
// invalid) in the SearchResults shape of the agile issue lists.
func writeEpicIssuePage(w http.ResponseWriter, r *http.Request, issueH *IssueHandler, issueSvc *issue.Service, uid string, opts ...issue.ListOption) {
	order := ""
	if jqlStr := strings.TrimSpace(r.URL.Query().Get("jql")); jqlStr != "" {
		q, err := jql.Parse(jqlStr)
		if err != nil {
			v3.WriteError(w, http.StatusBadRequest, []string{"invalid jql: " + err.Error()}, nil)
			return
		}
		compiled, err := jql.Compile(q, search.NewDBResolver(issueSvc.DB(), uid))
		if err != nil {
			v3.WriteError(w, http.StatusBadRequest, []string{"invalid jql: " + err.Error()}, nil)
			return
		}
		opts = append(opts, issue.WithWhere(compiled.Where, compiled.Args...))
		order = compiled.Order
	}
	startAt, maxResults := v3.ParsePagination(r, 50, 100)
	issues, total, err := issueSvc.ListPage(startAt, maxResults, order, opts...)
	if err != nil {
		v3.WriteError(w, http.StatusInternalServerError, []string{"failed to list issues"}, nil)
		return
	}
	items, err := renderIssueList(issueH, issues, nil)
	if err != nil {
		v3.WriteError(w, http.StatusInternalServerError, []string{"render error"}, nil)
		return
	}
	v3.WriteJSON(w, http.StatusOK, v3.SearchResults{Issues: items, StartAt: startAt, MaxResults: maxResults, Total: total})
}

// EpicIssues: GET /rest/agile/1.0/epic/{epicIdOrKey}/issue — the epic's
// children (level-0, non-archived), paginated, optional jql.
func (h *AgileMiscHandler) EpicIssues(w http.ResponseWriter, r *http.Request) {
	epic := h.resolveEpic(r.PathValue("epicIdOrKey"))
	if epic == nil {
		v3.WriteError(w, http.StatusNotFound, []string{"epic not found"}, nil)
		return
	}
	uid := middleware.UserIDFromContext(r.Context())
	writeEpicIssuePage(w, r, h.issueH, h.issueSvc, uid, issue.WithParent(epic.ID))
}

// EpicNoneIssues: GET /rest/agile/1.0/epic/none/issue — level-0 issues that
// belong to no epic. Jira's is instance-wide; here it spans the projects the
// caller is a member of (same scope as /search/jql: unscoped for a global
// admin), narrowed by the optional jql.
func (h *AgileMiscHandler) EpicNoneIssues(w http.ResponseWriter, r *http.Request) {
	uid := middleware.UserIDFromContext(r.Context())
	var scope *gorm.DB
	if h.chk != nil && h.projectSvc != nil && !h.chk.IsGlobalAdmin(uid) {
		scope = h.projectSvc.MembershipSubquery(uid)
	}
	writeEpicIssuePage(w, r, h.issueH, h.issueSvc, uid,
		issue.WithProjectScope(scope), issue.WithNoParent(), issue.WithHierarchyLevel(0))
}

// readIssuesBody decodes the {issues: [key or id]} body of the move/remove
// routes; 400 when it is missing or empty.
func readIssuesBody(w http.ResponseWriter, r *http.Request) ([]string, bool) {
	var req struct {
		Issues []string `json:"issues"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		v3.WriteError(w, http.StatusBadRequest, []string{"invalid request body"}, nil)
		return nil, false
	}
	if len(req.Issues) == 0 {
		v3.WriteError(w, http.StatusBadRequest, []string{"issues is required"}, nil)
		return nil, false
	}
	return req.Issues, true
}

const epicChildMsg = "issues must be stories, tasks or bugs of the epic's project"

// MoveToEpic: POST /rest/agile/1.0/epic/{epicIdOrKey}/issue. The path
// authorizes EditIssues on the EPIC's project; the issues come from the body
// and are only authorized through that same project: each must resolve, belong
// to the epic's project and be a level-0 issue (a subtask, an epic, an issue of
// another project or an unknown key/id all make the request invalid → 400,
// one message for every case so a guessed key reveals nothing). The whole
// batch is validated before the first write (no partial move).
func (h *AgileMiscHandler) MoveToEpic(w http.ResponseWriter, r *http.Request) {
	epic := h.resolveEpic(r.PathValue("epicIdOrKey"))
	if epic == nil {
		v3.WriteError(w, http.StatusNotFound, []string{"epic not found"}, nil)
		return
	}
	keys, ok := readIssuesBody(w, r)
	if !ok {
		return
	}
	seen := map[string]bool{}
	targets := make([]*issue.Issue, 0, len(keys))
	for _, k := range keys {
		iss := h.resolveIssue(k)
		if iss == nil || iss.ProjectID != epic.ProjectID || iss.IsArchived || issueLevel(h.issueSvc.DB(), iss) != 0 {
			v3.WriteError(w, http.StatusBadRequest, []string{epicChildMsg}, nil)
			return
		}
		if seen[iss.ID] {
			continue
		}
		seen[iss.ID] = true
		targets = append(targets, iss)
	}
	for _, iss := range targets {
		if err := h.issueSvc.SetParent(iss.Key, &epic.ID); err != nil {
			if errors.Is(err, issue.ErrParentInvalid) || errors.Is(err, issue.ErrParentNotFound) {
				v3.WriteError(w, http.StatusBadRequest, []string{epicChildMsg}, nil)
				return
			}
			v3.WriteError(w, http.StatusInternalServerError, []string{"failed to move issue"}, nil)
			return
		}
	}
	w.WriteHeader(http.StatusNoContent)
}

// RemoveFromEpic: POST /rest/agile/1.0/epic/none/issue — clears the parent
// of each body issue. No path context: every issue needs EditIssues on its
// own project (404 when it does not resolve or is not browsable, 403 when
// browsable but not editable); a subtask is not in an epic → 400. The batch
// is validated as a whole before the first write.
func (h *AgileMiscHandler) RemoveFromEpic(w http.ResponseWriter, r *http.Request) {
	keys, ok := readIssuesBody(w, r)
	if !ok {
		return
	}
	uid := middleware.UserIDFromContext(r.Context())
	issues, status, msg := h.resolveBodyIssues(uid, keys, permission.EditIssues)
	if status != 0 {
		v3.WriteError(w, status, []string{msg}, nil)
		return
	}
	for _, iss := range issues {
		if issueLevel(h.issueSvc.DB(), iss) != 0 {
			v3.WriteError(w, http.StatusBadRequest, []string{"issues must be stories, tasks or bugs"}, nil)
			return
		}
	}
	for _, iss := range issues {
		if err := h.issueSvc.SetParent(iss.Key, nil); err != nil {
			v3.WriteError(w, http.StatusInternalServerError, []string{"failed to remove issue from epic"}, nil)
			return
		}
	}
	w.WriteHeader(http.StatusNoContent)
}

// RankEpic: PUT /rest/agile/1.0/epic/{epicIdOrKey}/rank — moves the epic
// right before/after another epic of the same project (exactly one anchor;
// rankCustomFieldId is accepted and ignored: Heureum has one rank). Reuses
// the issue rank (position midpoint). The path authorizes ManageSprints like
// PUT /issue/rank.
func (h *AgileMiscHandler) RankEpic(w http.ResponseWriter, r *http.Request) {
	epic := h.resolveEpic(r.PathValue("epicIdOrKey"))
	if epic == nil {
		v3.WriteError(w, http.StatusNotFound, []string{"epic not found"}, nil)
		return
	}
	var req struct {
		RankBeforeEpic    string `json:"rankBeforeEpic"`
		RankAfterEpic     string `json:"rankAfterEpic"`
		RankCustomFieldID int64  `json:"rankCustomFieldId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		v3.WriteError(w, http.StatusBadRequest, []string{"invalid request body"}, nil)
		return
	}
	if (req.RankBeforeEpic == "") == (req.RankAfterEpic == "") {
		v3.WriteError(w, http.StatusBadRequest, []string{"exactly one of rankBeforeEpic or rankAfterEpic is required"}, nil)
		return
	}
	ref := req.RankBeforeEpic
	if ref == "" {
		ref = req.RankAfterEpic
	}
	anchor := h.resolveEpic(ref)
	if anchor == nil || anchor.ProjectID != epic.ProjectID || anchor.ID == epic.ID {
		v3.WriteError(w, http.StatusBadRequest, []string{"rankBeforeEpic/rankAfterEpic must be another epic of the same project"}, nil)
		return
	}
	// "Immediately before/after the anchor": both bounds are passed to Rank
	// (the anchor and its neighbouring epic on the other side, if any) so the
	// new position is the midpoint between them — with one bound only, Rank
	// would step a fixed distance away from the anchor and could land on an
	// epic sitting in between.
	before, after := h.epicRankBounds(epic, anchor, req.RankBeforeEpic != "")
	if err := h.issueSvc.Rank([]string{epic.ID}, before, after); err != nil {
		v3.WriteError(w, http.StatusInternalServerError, []string{"failed to rank epic"}, nil)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// epicRankBounds returns the (before, after) ids to hand to Rank for moving
// epic right before (beforeAnchor=true) or after the anchor: the anchor on one
// side and, on the other, the project's epic adjacent to the anchor in the
// current position order (nil at the ends), the moved epic excluded.
func (h *AgileMiscHandler) epicRankBounds(epic, anchor *issue.Issue, beforeAnchor bool) (before, after *string) {
	epics, err := h.issueSvc.ListByProject(epic.ProjectID, issue.WithHierarchyLevel(1))
	if err != nil {
		if beforeAnchor {
			return &anchor.ID, nil
		}
		return nil, &anchor.ID
	}
	ordered := make([]string, 0, len(epics))
	for i := range epics {
		if epics[i].ID != epic.ID {
			ordered = append(ordered, epics[i].ID)
		}
	}
	idx := -1
	for i, id := range ordered {
		if id == anchor.ID {
			idx = i
			break
		}
	}
	if beforeAnchor {
		if idx > 0 {
			after = &ordered[idx-1]
		}
		return &anchor.ID, after
	}
	if idx >= 0 && idx+1 < len(ordered) {
		before = &ordered[idx+1]
	}
	return before, &anchor.ID
}

// UpdateEpic: POST /rest/agile/1.0/epic/{epicIdOrKey} — partial update
// {name?, summary?, done?, color{key}?}. Heureum keeps ONE title per issue, so
// name and summary both set it (summary wins when both are sent and differ);
// an empty title is 400. color.key must be one of issue.EpicColorKeys (400).
// done moves the epic through its workflow to the first status of category
// done (true) / todo (false) reachable by ONE available transition — the same
// path as POST /issue/{key}/transitions, validators and post-functions
// included — 409 when none is reachable, no-op when already there; it needs
// TransitionIssues on top of the route's EditIssues. Everything is validated
// before the first write. Returns the epic shape.
func (h *AgileMiscHandler) UpdateEpic(w http.ResponseWriter, r *http.Request) {
	epic := h.resolveEpic(r.PathValue("epicIdOrKey"))
	if epic == nil {
		v3.WriteError(w, http.StatusNotFound, []string{"epic not found"}, nil)
		return
	}
	var req struct {
		Name    *string `json:"name"`
		Summary *string `json:"summary"`
		Done    *bool   `json:"done"`
		Color   *struct {
			Key string `json:"key"`
		} `json:"color"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		v3.WriteError(w, http.StatusBadRequest, []string{"invalid request body"}, nil)
		return
	}
	title := req.Summary
	if title == nil {
		title = req.Name
	}
	if title != nil && strings.TrimSpace(*title) == "" {
		v3.WriteError(w, http.StatusBadRequest, []string{"summary must not be empty"}, map[string]string{"summary": "You must specify a summary of the issue."})
		return
	}
	if req.Color != nil && !issue.IsEpicColorKey(req.Color.Key) {
		v3.WriteError(w, http.StatusBadRequest, []string{"color.key must be one of " + strings.Join(issue.EpicColorKeys, ", ")}, map[string]string{"color": "invalid colour key"})
		return
	}

	var (
		wf *workflow.Workflow
		tr *workflow.WorkflowTransition
	)
	if req.Done != nil {
		uid := middleware.UserIDFromContext(r.Context())
		if h.chk != nil && h.chk.RequireProject(uid, epic.ProjectID, permission.TransitionIssues) != nil {
			authz.WriteForbidden(w)
			return
		}
		var status int
		var msg string
		wf, tr, status, msg = h.doneTransition(epic, *req.Done)
		if status != 0 {
			v3.WriteError(w, status, []string{msg}, nil)
			return
		}
	}

	if title != nil && *title != epic.Title {
		if _, err := h.issueSvc.Update(epic.Key, title, nil, nil, nil, nil, nil); err != nil {
			v3.WriteError(w, http.StatusInternalServerError, []string{"failed to update epic"}, nil)
			return
		}
	}
	if req.Color != nil {
		if err := h.issueSvc.SetEpicColor(epic.Key, req.Color.Key); err != nil {
			v3.WriteError(w, http.StatusInternalServerError, []string{"failed to update epic colour"}, nil)
			return
		}
	}
	if tr != nil {
		if status, msg := transitionIssue(h.issueSvc, wf, epic, tr); status != 0 {
			v3.WriteError(w, status, []string{msg}, nil)
			return
		}
	}
	updated, err := h.issueSvc.GetByKey(epic.Key)
	if err != nil {
		v3.WriteError(w, http.StatusInternalServerError, []string{"failed to load epic"}, nil)
		return
	}
	v3.WriteJSON(w, http.StatusOK, epicBean(h.baseURL, updated, isDoneStatus(h.issueSvc.DB(), updated.StatusID)))
}

// doneTransition picks the transition that takes epic to the first status
// (by workflow position) of category done (done=true) or todo (done=false)
// among the transitions available from its current status. Returns a nil
// transition with status 0 when the epic already sits in that category
// (no-op), 409 when no such status is one transition away, 400 when the
// transition's validator blocks it (checked here so nothing is written).
func (h *AgileMiscHandler) doneTransition(epic *issue.Issue, done bool) (*workflow.Workflow, *workflow.WorkflowTransition, int, string) {
	if h.wfSvc == nil {
		return nil, nil, http.StatusConflict, "no workflow available to change the epic's status"
	}
	wf, err := h.wfSvc.GetWorkflow(epic.ProjectID)
	if err != nil {
		return nil, nil, http.StatusConflict, "no workflow available to change the epic's status"
	}
	want := workflow.CategoryTodo
	if done {
		want = workflow.CategoryDone
	}
	from := ""
	if epic.StatusID != nil {
		from = *epic.StatusID
		if cur := statusByID(wf, from); cur != nil && cur.Category == want {
			return wf, nil, 0, ""
		}
	}
	avail, err := h.wfSvc.GetAvailableTransitions(wf.ID, from)
	if err != nil {
		return nil, nil, http.StatusInternalServerError, "failed to list transitions"
	}
	type candidate struct {
		tr  workflow.WorkflowTransition
		pos int
	}
	var cands []candidate
	for _, t := range avail {
		if to := statusByID(wf, t.ToStatusID); to != nil && to.Category == want {
			cands = append(cands, candidate{tr: t, pos: to.Position})
		}
	}
	if len(cands) == 0 {
		label := "to do"
		if done {
			label = "done"
		}
		return nil, nil, http.StatusConflict, "no available transition takes the epic to a " + label + " status"
	}
	sort.SliceStable(cands, func(i, j int) bool { return cands[i].pos < cands[j].pos })
	tr := cands[0].tr
	if msg := transitionBlocked(epic, &tr); msg != "" {
		return nil, nil, http.StatusBadRequest, msg
	}
	return wf, &tr, 0, ""
}
