package handlers

import (
	"net/http"
	"testing"

	"github.com/it4nodummies/heureum/internal/domain/issue"
	"github.com/it4nodummies/heureum/internal/domain/project"
)

// epicFixture adds an epic and a story to each project of the nested fixture.
type epicFixture struct {
	*nestedFixture
	epicA, storyA, epicB, storyB *issue.Issue
}

func setupEpicFixture(t *testing.T) *epicFixture {
	t.Helper()
	f := &epicFixture{nestedFixture: setupNestedFixture(t)}
	mk := func(p *project.Project, title, typeName string) *issue.Issue {
		t.Helper()
		typeID, err := f.issueSvc.TypeIDByName(p.ID, typeName)
		if err != nil {
			t.Fatal(err)
		}
		iss, err := f.issueSvc.Create(p.Key, p.ID, title, "", issue.PriorityMedium, nil, &typeID)
		if err != nil {
			t.Fatalf("create %s in %s: %v", typeName, p.Key, err)
		}
		return iss
	}
	f.epicA = mk(f.projA, "epic A", "Epic")
	f.storyA = mk(f.projA, "story A", "Story")
	f.epicB = mk(f.projB, "epic B", "Epic")
	f.storyB = mk(f.projB, "story B", "Story")
	return f
}

// TestUpdateEpic_RequiresEditIssues: POST /epic/{id} is an issue edit. A
// viewer of the epic's project gets 403, so does a user with no role there
// (chk.Enforce, the convention of every path-gated mutation), a member 200.
func TestUpdateEpic_RequiresEditIssues(t *testing.T) {
	f := setupEpicFixture(t)
	body := `{"name":"renamed","color":{"key":"color_3"}}`

	f.expect(t, "viewer of B updates B's epic", f.do(t, f.adminA, http.MethodPost, "/rest/agile/1.0/epic/"+f.epicB.Key, body), http.StatusForbidden)
	f.expect(t, "no role in B updates B's epic", f.do(t, f.adminAOnly, http.MethodPost, "/rest/agile/1.0/epic/"+f.epicB.Key, body), http.StatusForbidden)
	if got := f.reloadIssue(t, f.epicB.ID); got.Title != "epic B" || (got.EpicColor != nil && *got.EpicColor == "color_3") {
		t.Errorf("epic B written by a rejected update: title=%q colour=%v", got.Title, got.EpicColor)
	}
	f.expect(t, "member of B updates B's epic", f.do(t, f.memberB, http.MethodPost, "/rest/agile/1.0/epic/"+f.epicB.Key, body), http.StatusOK)
	if got := f.reloadIssue(t, f.epicB.ID); got.Title != "renamed" || got.EpicColor == nil || *got.EpicColor != "color_3" {
		t.Errorf("epic B not updated by member: title=%q colour=%v", got.Title, got.EpicColor)
	}
	// A story is not an epic, even for its admin.
	f.expect(t, "POST /epic on a story", f.do(t, f.adminA, http.MethodPost, "/rest/agile/1.0/epic/"+f.storyA.Key, body), http.StatusNotFound)
}

// TestMoveToEpic_RequiresEditIssuesAndSameProject: the path authorizes
// EditIssues on the epic's project; every body issue must then belong to that
// same project and be a level-0 issue, else 400 with nothing moved.
func TestMoveToEpic_RequiresEditIssuesAndSameProject(t *testing.T) {
	f := setupEpicFixture(t)
	mkBody := func(keys ...string) string {
		s := `{"issues":[`
		for i, k := range keys {
			if i > 0 {
				s += ","
			}
			s += `"` + k + `"`
		}
		return s + `]}`
	}

	f.expect(t, "viewer of B moves into B's epic", f.do(t, f.adminA, http.MethodPost, "/rest/agile/1.0/epic/"+f.epicB.Key+"/issue", mkBody(f.storyB.Key)), http.StatusForbidden)
	f.expect(t, "no role in B moves into B's epic", f.do(t, f.adminAOnly, http.MethodPost, "/rest/agile/1.0/epic/"+f.epicB.Key+"/issue", mkBody(f.storyB.Key)), http.StatusForbidden)
	if got := f.reloadIssue(t, f.storyB.ID); got.ParentID != nil {
		t.Errorf("story B moved by a rejected request")
	}
	// Admin of A, viewer of B: B's story under A's epic is a cross-project
	// move → 400, and A's story (first in the batch) must not move either.
	f.expect(t, "cross-project batch", f.do(t, f.adminA, http.MethodPost, "/rest/agile/1.0/epic/"+f.epicA.Key+"/issue", mkBody(f.storyA.Key, f.storyB.Key)), http.StatusBadRequest)
	if got := f.reloadIssue(t, f.storyA.ID); got.ParentID != nil {
		t.Errorf("partial write: story A moved although the batch was rejected")
	}
	f.expect(t, "own project", f.do(t, f.adminA, http.MethodPost, "/rest/agile/1.0/epic/"+f.epicA.Key+"/issue", mkBody(f.storyA.Key)), http.StatusNoContent)
	if got := f.reloadIssue(t, f.storyA.ID); got.ParentID == nil || *got.ParentID != f.epicA.ID {
		t.Errorf("story A not moved under epic A: parent=%v", got.ParentID)
	}

	// Remove: no path context, so each body issue needs EditIssues on its own
	// project; the viewer of B gets 403 and nothing is cleared.
	f.expect(t, "viewer of B removes B's story", f.do(t, f.adminA, http.MethodPost, "/rest/agile/1.0/epic/none/issue", mkBody(f.storyA.Key, f.storyB.Key)), http.StatusForbidden)
	if got := f.reloadIssue(t, f.storyA.ID); got.ParentID == nil {
		t.Errorf("partial write: story A cleared although the batch was rejected")
	}
	f.expect(t, "no role in B removes B's story", f.do(t, f.adminAOnly, http.MethodPost, "/rest/agile/1.0/epic/none/issue", mkBody(f.storyB.Key)), http.StatusNotFound)
	f.expect(t, "remove own", f.do(t, f.adminA, http.MethodPost, "/rest/agile/1.0/epic/none/issue", mkBody(f.storyA.Key)), http.StatusNoContent)
	if got := f.reloadIssue(t, f.storyA.ID); got.ParentID != nil {
		t.Errorf("story A still parented after remove")
	}

	// Rank: ManageSprints on the epic's project (viewer → 403).
	f.expect(t, "viewer ranks B's epic", f.do(t, f.adminA, http.MethodPut, "/rest/agile/1.0/epic/"+f.epicB.Key+"/rank", `{"rankBeforeEpic":"`+f.epicB.Key+`"}`), http.StatusForbidden)
}
