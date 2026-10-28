package handlers

import (
	"net/http"
	"strings"
	"testing"

	"github.com/it4nodummies/heureum/internal/domain/issue"
)

func (f *nestedFixture) countLinks(t *testing.T) int64 {
	t.Helper()
	var cnt int64
	if err := f.db.Model(&issue.IssueLink{}).Count(&cnt).Error; err != nil {
		t.Fatal(err)
	}
	return cnt
}

func linkBody(outward, inward string) string {
	return `{"type":{"name":"Relates"},"outwardIssue":{"key":"` + outward + `"},"inwardIssue":{"key":"` + inward + `"}}`
}

// TestIssueLink_RequiresBrowseOnBothSides: POST /issueLink authorizes the
// OUTWARD issue only; the inward issue (B, where the caller has no role) was
// linked blindly. Both sides must be browsable by the caller: the inward side
// not being visible must yield the same 404 the outward side yields when it is
// not found (no existence leak), and no link row may be written. Linking two
// issues of A (and, for a caller who can browse both projects, A → B) is fine.
func TestIssueLink_RequiresBrowseOnBothSides(t *testing.T) {
	f := setupNestedFixture(t)

	// Caller admin of A, no role in B: inward B must be rejected.
	w := f.do(t, f.adminAOnly, http.MethodPost, "/rest/api/3/issueLink", linkBody(f.issueA.Key, f.issueB.Key))
	f.expect(t, "link A -> unbrowsable B", w, http.StatusNotFound)
	if n := f.countLinks(t); n != 0 {
		t.Errorf("link written across project boundary (rows=%d)", n)
	}
	// Mirror: outward B (unbrowsable) -> inward A: same status as "not found".
	w = f.do(t, f.adminAOnly, http.MethodPost, "/rest/api/3/issueLink", linkBody(f.issueB.Key, f.issueA.Key))
	f.expect(t, "link unbrowsable B -> A", w, http.StatusNotFound)
	if n := f.countLinks(t); n != 0 {
		t.Errorf("link written across project boundary (rows=%d)", n)
	}
	// A genuinely missing inward issue must look identical.
	w = f.do(t, f.adminAOnly, http.MethodPost, "/rest/api/3/issueLink", linkBody(f.issueA.Key, "NOPE-1"))
	f.expect(t, "link A -> missing", w, http.StatusNotFound)

	// Positive: both sides in A.
	issueA2, err := f.issueSvc.Create("AAA", f.projA.ID, "issue A2", "", issue.PriorityMedium, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	w = f.do(t, f.adminAOnly, http.MethodPost, "/rest/api/3/issueLink", linkBody(f.issueA.Key, issueA2.Key))
	f.expect(t, "link A -> A2", w, http.StatusCreated)
	if n := f.countLinks(t); n != 1 {
		t.Errorf("expected one link, got %d", n)
	}
	// Positive: a caller who can browse both projects may link across them
	// (adminA is admin of A and viewer of B).
	w = f.do(t, f.adminA, http.MethodPost, "/rest/api/3/issueLink", linkBody(f.issueA.Key, f.issueB.Key))
	f.expect(t, "link A -> B by a viewer of B", w, http.StatusCreated)
	if n := f.countLinks(t); n != 2 {
		t.Errorf("expected two links, got %d", n)
	}
}

// TestIssueLink_ReadDoesNotExposeUnbrowsableSide: a link A → B created by
// someone who can see both must not expose B's summary to a caller with no
// role in B: GET /issueLink/{id} → 404 and GET /issue/A/issuelinks omits it.
func TestIssueLink_ReadDoesNotExposeUnbrowsableSide(t *testing.T) {
	f := setupNestedFixture(t)
	link, err := f.issueSvc.AddLink(f.issueA.ID, f.issueB.ID, issue.LinkRelates)
	if err != nil {
		t.Fatal(err)
	}

	f.expect(t, "GET link with unbrowsable inward", f.do(t, f.adminAOnly, http.MethodGet, "/rest/api/3/issueLink/"+link.ID, ""), http.StatusNotFound)
	w := f.do(t, f.adminAOnly, http.MethodGet, "/rest/api/3/issue/"+f.issueA.Key+"/issuelinks", "")
	f.expect(t, "list links of A", w, http.StatusOK)
	if strings.Contains(w.Body.String(), f.issueB.Key) || strings.Contains(w.Body.String(), "issue B") {
		t.Errorf("issuelinks of A exposes unbrowsable issue B: %s", w.Body.String())
	}

	// DELETE is the same class (link row → both issues): not visible → 404,
	// row intact.
	f.expect(t, "DELETE link with unbrowsable inward", f.do(t, f.adminAOnly, http.MethodDelete, "/rest/api/3/issueLink/"+link.ID, ""), http.StatusNotFound)
	if n := f.countLinks(t); n != 1 {
		t.Errorf("link deleted by a caller who cannot see one side (rows=%d)", n)
	}

	// A caller who can browse both sees the link, and (Edit on the source
	// project A) may delete it.
	f.expect(t, "GET link as viewer of B", f.do(t, f.adminA, http.MethodGet, "/rest/api/3/issueLink/"+link.ID, ""), http.StatusOK)
	w = f.do(t, f.adminA, http.MethodGet, "/rest/api/3/issue/"+f.issueA.Key+"/issuelinks", "")
	if !strings.Contains(w.Body.String(), f.issueB.Key) {
		t.Errorf("issuelinks of A hides a browsable issue B: %s", w.Body.String())
	}
	f.expect(t, "DELETE link as viewer of B / admin of A", f.do(t, f.adminA, http.MethodDelete, "/rest/api/3/issueLink/"+link.ID, ""), http.StatusNoContent)
	if n := f.countLinks(t); n != 0 {
		t.Errorf("link not deleted (rows=%d)", n)
	}
}
