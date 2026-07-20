package issue

import (
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newIssueTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&Issue{}, &IssueType{}, &Label{}, &IssueLabel{}, &IssueHistory{}); err != nil {
		t.Fatal(err)
	}
	return db
}

func TestCreateAssignsSeqID(t *testing.T) {
	db := newIssueTestDB(t)
	svc := NewService(db)
	i1, err := svc.Create("DEMO", "p1", "First", "", PriorityMedium, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if i1.SeqID < 10000 {
		t.Errorf("seq_id = %d, want >= 10000", i1.SeqID)
	}
	i2, _ := svc.Create("DEMO", "p1", "Second", "", PriorityMedium, nil, nil)
	if i2.SeqID != i1.SeqID+1 {
		t.Errorf("second seq_id = %d, want %d", i2.SeqID, i1.SeqID+1)
	}
	got, err := svc.GetBySeqID(i1.SeqID)
	if err != nil || got.Key != i1.Key {
		t.Errorf("GetBySeqID: got %+v err %v", got, err)
	}
}

// TestUpdateEmptyAssigneeStoresNull verifies that unassigning an issue (an empty
// accountId reaching Update) writes SQL NULL to the nullable assignee_id FK
// column, not "". Before the fix the map wrote *assigneeID ("") which is silently
// accepted on SQLite but violates the FK on Postgres (SQLSTATE 23503).
func TestUpdateEmptyAssigneeStoresNull(t *testing.T) {
	db := newIssueTestDB(t)
	svc := NewService(db)
	iss, err := svc.Create("DEMO", "p1", "First", "", PriorityMedium, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	// Assign someone first so the update is a genuine unassign.
	const uid = "user-123"
	if err := db.Model(&Issue{}).Where("key = ?", iss.Key).Update("assignee_id", uid).Error; err != nil {
		t.Fatal(err)
	}
	if got, _ := svc.GetByKey(iss.Key); got.AssigneeID == nil || *got.AssigneeID != uid {
		t.Fatalf("setup: assignee not set, got %v", got.AssigneeID)
	}

	title := "First"
	empty := ""
	if _, err := svc.Update(iss.Key, &title, nil, nil, &empty, nil, nil); err != nil {
		t.Fatal(err)
	}

	got, err := svc.GetByKey(iss.Key)
	if err != nil {
		t.Fatal(err)
	}
	if got.AssigneeID != nil {
		t.Errorf("AssigneeID = %q, want nil (unassigned)", *got.AssigneeID)
	}
}

// TestLogHistoryEmptyActorStoresNull verifies that an Update recording history
// with a blank actor (the current callers pass "") produces an issue_history row
// whose actor_id is SQL NULL, not "" — the latter violates the nullable actor_id
// FK on Postgres, which is why the changelog stayed empty there.
func TestLogHistoryEmptyActorStoresNull(t *testing.T) {
	db := newIssueTestDB(t)
	svc := NewService(db)
	iss, err := svc.Create("DEMO", "p1", "First", "", PriorityMedium, nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	newTitle := "Renamed"
	if _, err := svc.Update(iss.Key, &newTitle, nil, nil, nil, nil, nil); err != nil {
		t.Fatal(err)
	}

	history, err := svc.GetHistory(iss.ID)
	if err != nil {
		t.Fatal(err)
	}
	var titleRow *IssueHistory
	for i := range history {
		if history[i].FieldName == "title" {
			titleRow = &history[i]
			break
		}
	}
	if titleRow == nil {
		t.Fatalf("no 'title' history row recorded; got %d rows", len(history))
	}
	if titleRow.ActorID != nil {
		t.Errorf("ActorID = %q, want nil (blank actor)", *titleRow.ActorID)
	}
}
