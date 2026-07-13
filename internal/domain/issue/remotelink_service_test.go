package issue

import "testing"

func TestRemoteLinkService_AddAndList(t *testing.T) {
	db := newIssueTestDB(t)
	if err := db.AutoMigrate(&RemoteLink{}); err != nil {
		t.Fatal(err)
	}
	svc := NewRemoteLinkService(db)

	rl, err := svc.Add("issue-1", "system=http://acme.com&id=1", "https://example.com/doc", "Doc", "A summary", "causes")
	if err != nil {
		t.Fatal(err)
	}
	if rl.ID == "" {
		t.Error("ID = empty, want generated UUID")
	}
	if rl.IssueID != "issue-1" {
		t.Errorf("IssueID = %q, want issue-1", rl.IssueID)
	}

	links, err := svc.ListByIssue("issue-1")
	if err != nil {
		t.Fatal(err)
	}
	if len(links) != 1 {
		t.Fatalf("ListByIssue len = %d, want 1", len(links))
	}
	if links[0].GlobalID != "system=http://acme.com&id=1" {
		t.Errorf("GlobalID = %q", links[0].GlobalID)
	}
	if links[0].URL != "https://example.com/doc" {
		t.Errorf("URL = %q", links[0].URL)
	}
	if links[0].Title != "Doc" {
		t.Errorf("Title = %q", links[0].Title)
	}
	if links[0].Summary != "A summary" {
		t.Errorf("Summary = %q", links[0].Summary)
	}
	if links[0].Relationship != "causes" {
		t.Errorf("Relationship = %q", links[0].Relationship)
	}
}

func TestRemoteLinkService_Delete(t *testing.T) {
	db := newIssueTestDB(t)
	if err := db.AutoMigrate(&RemoteLink{}); err != nil {
		t.Fatal(err)
	}
	svc := NewRemoteLinkService(db)

	rl, err := svc.Add("issue-1", "", "https://example.com/doc", "Doc", "", "")
	if err != nil {
		t.Fatal(err)
	}

	// Delete scoped to a different issue must not remove the link and must
	// report zero rows affected.
	n, err := svc.Delete("issue-2", rl.ID)
	if err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Errorf("Delete with wrong issueID RowsAffected = %d, want 0", n)
	}
	links, err := svc.ListByIssue("issue-1")
	if err != nil {
		t.Fatal(err)
	}
	if len(links) != 1 {
		t.Fatalf("ListByIssue after mismatched Delete len = %d, want 1", len(links))
	}

	// Delete scoped to the owning issue removes the link.
	n, err = svc.Delete("issue-1", rl.ID)
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Errorf("Delete RowsAffected = %d, want 1", n)
	}
	links, err = svc.ListByIssue("issue-1")
	if err != nil {
		t.Fatal(err)
	}
	if len(links) != 0 {
		t.Errorf("ListByIssue after Delete len = %d, want 0", len(links))
	}
}

func TestRemoteLinkService_ListByIssueEmpty(t *testing.T) {
	db := newIssueTestDB(t)
	if err := db.AutoMigrate(&RemoteLink{}); err != nil {
		t.Fatal(err)
	}
	svc := NewRemoteLinkService(db)

	links, err := svc.ListByIssue("no-such-issue")
	if err != nil {
		t.Fatal(err)
	}
	if len(links) != 0 {
		t.Errorf("ListByIssue len = %d, want 0", len(links))
	}
}
