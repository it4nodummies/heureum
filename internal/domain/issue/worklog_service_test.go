package issue

import "testing"

func TestWorklogService_AddListDelete(t *testing.T) {
	db := newIssueTestDB(t)
	if err := db.AutoMigrate(&Worklog{}); err != nil {
		t.Fatal(err)
	}
	svc := NewWorklogService(db)

	wl, err := svc.Add("issue-1", "user-1", `{"type":"doc","version":1,"content":[]}`, 3600)
	if err != nil {
		t.Fatal(err)
	}
	if wl.ID == "" {
		t.Error("expected generated ID")
	}
	if wl.IssueID != "issue-1" {
		t.Errorf("IssueID = %q, want issue-1", wl.IssueID)
	}
	if wl.AuthorID == nil || *wl.AuthorID != "user-1" {
		t.Errorf("AuthorID = %v, want user-1", wl.AuthorID)
	}
	if wl.TimeSpentSeconds != 3600 {
		t.Errorf("TimeSpentSeconds = %d, want 3600", wl.TimeSpentSeconds)
	}
	if wl.Started == nil {
		t.Error("expected Started to be set")
	}

	list, err := svc.ListByIssue("issue-1")
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 {
		t.Fatalf("ListByIssue len = %d, want 1", len(list))
	}

	got, err := svc.Get(wl.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != wl.ID {
		t.Errorf("Get id = %q, want %q", got.ID, wl.ID)
	}

	if err := svc.Delete(wl.ID); err != nil {
		t.Fatal(err)
	}
	list, err = svc.ListByIssue("issue-1")
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 0 {
		t.Errorf("ListByIssue after delete len = %d, want 0", len(list))
	}
}

func TestWorklogService_AddDefaultsEmptyComment(t *testing.T) {
	db := newIssueTestDB(t)
	if err := db.AutoMigrate(&Worklog{}); err != nil {
		t.Fatal(err)
	}
	svc := NewWorklogService(db)

	wl, err := svc.Add("issue-1", "", "", 60)
	if err != nil {
		t.Fatal(err)
	}
	if wl.CommentJSON != "{}" {
		t.Errorf("CommentJSON = %q, want {}", wl.CommentJSON)
	}
	if wl.AuthorID != nil {
		t.Errorf("AuthorID = %v, want nil", wl.AuthorID)
	}
}
