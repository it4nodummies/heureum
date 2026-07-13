package issue

import "testing"

func TestVoteService_AddIsIdempotent(t *testing.T) {
	db := newIssueTestDB(t)
	if err := db.AutoMigrate(&Vote{}); err != nil {
		t.Fatal(err)
	}
	svc := NewVoteService(db)

	if err := svc.Add("issue-1", "user-1"); err != nil {
		t.Fatal(err)
	}
	if err := svc.Add("issue-1", "user-1"); err != nil {
		t.Fatal(err)
	}
	if got := svc.Count("issue-1"); got != 1 {
		t.Errorf("Count = %d, want 1", got)
	}
}

func TestVoteService_HasVoted(t *testing.T) {
	db := newIssueTestDB(t)
	if err := db.AutoMigrate(&Vote{}); err != nil {
		t.Fatal(err)
	}
	svc := NewVoteService(db)

	if svc.HasVoted("issue-1", "user-1") {
		t.Error("HasVoted = true before Add, want false")
	}
	if err := svc.Add("issue-1", "user-1"); err != nil {
		t.Fatal(err)
	}
	if !svc.HasVoted("issue-1", "user-1") {
		t.Error("HasVoted = false after Add, want true")
	}
}

func TestVoteService_Voters(t *testing.T) {
	db := newIssueTestDB(t)
	if err := db.AutoMigrate(&Vote{}); err != nil {
		t.Fatal(err)
	}
	svc := NewVoteService(db)

	if err := svc.Add("issue-1", "user-1"); err != nil {
		t.Fatal(err)
	}
	voters := svc.Voters("issue-1")
	if len(voters) != 1 || voters[0] != "user-1" {
		t.Errorf("Voters = %v, want [user-1]", voters)
	}
}

func TestVoteService_Remove(t *testing.T) {
	db := newIssueTestDB(t)
	if err := db.AutoMigrate(&Vote{}); err != nil {
		t.Fatal(err)
	}
	svc := NewVoteService(db)

	if err := svc.Add("issue-1", "user-1"); err != nil {
		t.Fatal(err)
	}
	if err := svc.Remove("issue-1", "user-1"); err != nil {
		t.Fatal(err)
	}
	if got := svc.Count("issue-1"); got != 0 {
		t.Errorf("Count after Remove = %d, want 0", got)
	}
	if svc.HasVoted("issue-1", "user-1") {
		t.Error("HasVoted after Remove = true, want false")
	}
}
