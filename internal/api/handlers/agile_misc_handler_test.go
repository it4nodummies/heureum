package handlers

import (
	"net/http"
	"strconv"
	"testing"
)

// TestMoveToBacklog_RequiresManageSprintsOnEveryIssue: POST /backlog/issue has
// no path context; it used to authorize ManageSprints on the FIRST body issue
// only, so [issueA, issueB] cleared B's sprint for an admin of A who is only a
// viewer of B. Every issue must be authorized and nothing written on failure.
func TestMoveToBacklog_RequiresManageSprintsOnEveryIssue(t *testing.T) {
	f := setupNestedFixture(t)
	spA, err := f.sprintSvc.Create(f.projA.ID, "Sprint A", "")
	if err != nil {
		t.Fatal(err)
	}
	spB, err := f.sprintSvc.Create(f.projB.ID, "Sprint B", "")
	if err != nil {
		t.Fatal(err)
	}
	if err := f.sprintSvc.AddIssue(spA.ID, f.issueA.ID); err != nil {
		t.Fatal(err)
	}
	if err := f.sprintSvc.AddIssue(spB.ID, f.issueB.ID); err != nil {
		t.Fatal(err)
	}

	// Viewer of B: visible but not manageable → 403, and A's issue (first in
	// the batch) must not have been moved either.
	w := f.do(t, f.adminA, http.MethodPost, "/rest/agile/1.0/backlog/issue", `{"issues":["`+f.issueA.Key+`","`+f.issueB.Key+`"]}`)
	f.expect(t, "backlog mixed batch as viewer of B", w, http.StatusForbidden)
	if got := f.reloadIssue(t, f.issueA.ID); got.SprintID == nil {
		t.Errorf("partial write: issue A moved to backlog although the batch was rejected")
	}
	if got := f.reloadIssue(t, f.issueB.ID); got.SprintID == nil {
		t.Errorf("issue B moved to backlog across project boundary")
	}
	// No role in B at all: must not learn that B's issue exists → 404, same as
	// a missing key.
	w = f.do(t, f.adminAOnly, http.MethodPost, "/rest/agile/1.0/backlog/issue", `{"issues":["`+f.issueA.Key+`","`+strconv.FormatInt(f.issueB.SeqID, 10)+`"]}`)
	f.expect(t, "backlog mixed batch with no role in B", w, http.StatusNotFound)
	w = f.do(t, f.adminAOnly, http.MethodPost, "/rest/agile/1.0/backlog/issue", `{"issues":["NOPE-1"]}`)
	f.expect(t, "backlog missing issue", w, http.StatusNotFound)
	if got := f.reloadIssue(t, f.issueA.ID); got.SprintID == nil {
		t.Errorf("partial write: issue A moved to backlog although the batch was rejected")
	}

	// Positive: own project.
	w = f.do(t, f.adminA, http.MethodPost, "/rest/agile/1.0/backlog/issue", `{"issues":["`+f.issueA.Key+`"]}`)
	f.expect(t, "backlog own issue", w, http.StatusNoContent)
	if got := f.reloadIssue(t, f.issueA.ID); got.SprintID != nil {
		t.Errorf("issue A not moved to backlog")
	}
}

// TestRank_RequiresManageSprintsOnEveryIssue: PUT /issue/rank authorized the
// first body issue only; rankBeforeIssue/rankAfterIssue were never checked.
// Every issue in the body (including the anchors) must be authorized and no
// position written on failure.
func TestRank_RequiresManageSprintsOnEveryIssue(t *testing.T) {
	f := setupNestedFixture(t)
	posA := f.reloadIssue(t, f.issueA.ID).Position
	posB := f.reloadIssue(t, f.issueB.ID).Position

	w := f.do(t, f.adminA, http.MethodPut, "/rest/agile/1.0/issue/rank", `{"issues":["`+f.issueA.Key+`","`+f.issueB.Key+`"]}`)
	f.expect(t, "rank mixed batch as viewer of B", w, http.StatusForbidden)
	// Anchors are body issues too.
	w = f.do(t, f.adminA, http.MethodPut, "/rest/agile/1.0/issue/rank", `{"issues":["`+f.issueA.Key+`"],"rankAfterIssue":"`+f.issueB.Key+`"}`)
	f.expect(t, "rank with other-project anchor", w, http.StatusForbidden)
	w = f.do(t, f.adminAOnly, http.MethodPut, "/rest/agile/1.0/issue/rank", `{"issues":["`+f.issueA.Key+`"],"rankBeforeIssue":"`+f.issueB.Key+`"}`)
	f.expect(t, "rank with unbrowsable anchor", w, http.StatusNotFound)
	if got := f.reloadIssue(t, f.issueA.ID); got.Position != posA {
		t.Errorf("partial write: issue A re-ranked although the batch was rejected (%v -> %v)", posA, got.Position)
	}
	if got := f.reloadIssue(t, f.issueB.ID); got.Position != posB {
		t.Errorf("issue B re-ranked across project boundary (%v -> %v)", posB, got.Position)
	}

	// Positive: own project.
	w = f.do(t, f.adminA, http.MethodPut, "/rest/agile/1.0/issue/rank", `{"issues":["`+f.issueA.Key+`"]}`)
	f.expect(t, "rank own issue", w, http.StatusNoContent)
}
