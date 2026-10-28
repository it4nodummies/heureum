package issue

import (
	"encoding/json"
	"reflect"
	"sync/atomic"
	"testing"

	"github.com/it4nodummies/heureum/internal/adf"
	"github.com/it4nodummies/heureum/internal/storetest"
	"gorm.io/gorm"
)

func newIssueTestDB(t *testing.T) *gorm.DB {
	return storetest.New(t)
}

// testSeqIDCounter hands out unique issues.seq_id values for helpers in this
// package's tests that insert Issue rows directly (bypassing Service.Create,
// which already guarantees uniqueness via nextSeqID) — issues.seq_id carries a
// real unique index in the migrated schema.
var testSeqIDCounter int64

func nextTestSeqID() int64 {
	return atomic.AddInt64(&testSeqIDCounter, 1) + 50000
}

// createTestProject inserts a minimal, valid projects row. The real schema
// (unlike the AutoMigrate stubs these tests used to run against) has
// issues.project_id, issue_types.project_id and labels.project_id as NOT NULL
// foreign keys to projects(id) — tests that create those rows directly must
// first create the project they claim to belong to. Idempotent, so callers
// that share a project id across several setup calls don't need to track
// whether it already exists.
func createTestProject(t *testing.T, db *gorm.DB, id string) {
	t.Helper()
	if err := db.Exec(
		`INSERT INTO projects (id, name, key) VALUES (?, ?, ?) ON CONFLICT (id) DO NOTHING`,
		id, id, id,
	).Error; err != nil {
		t.Fatalf("createTestProject(%q): %v", id, err)
	}
}

// createTestUser inserts a minimal, valid users row. Several nullable FKs
// (assignee_id, actor_id, uploader_id, author_id, ...) reference users(id):
// a test that assigns one of those columns to a literal id directly (rather
// than through a service call that only ever writes ids of rows it created)
// must first create that user.
func createTestUser(t *testing.T, db *gorm.DB, id string) {
	t.Helper()
	if err := db.Exec(
		`INSERT INTO users (id, email, username) VALUES (?, ?, ?) ON CONFLICT (id) DO NOTHING`,
		id, id+"@example.test", id,
	).Error; err != nil {
		t.Fatalf("createTestUser(%q): %v", id, err)
	}
}

// createTestIssue creates the project it belongs to (if not already present)
// and a minimal, valid issues row with the given id. Several tests in this
// package (worklogs, votes, remote links, attachments) reference a hardcoded
// issue id like "issue-1" directly instead of going through Service.Create;
// issues.project_id is a NOT NULL FK in the real schema, so that issue row
// must actually exist.
func createTestIssue(t *testing.T, db *gorm.DB, issueID, projectID string) {
	t.Helper()
	createTestProject(t, db, projectID)
	if err := db.Create(&Issue{
		ID:        issueID,
		ProjectID: projectID,
		Key:       issueID,
		Title:     "test issue",
		SeqID:     nextTestSeqID(),
	}).Error; err != nil {
		t.Fatalf("createTestIssue(%q): %v", issueID, err)
	}
}

func TestCreateAssignsSeqID(t *testing.T) {
	db := newIssueTestDB(t)
	createTestProject(t, db, "p1")
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

// TestSeedStandardTypesCreatesAndIsIdempotent verifies that SeedStandardTypes
// creates one issue type per name (inferring icon/isSubtask from the name via
// TypeIDByName) and that re-running it is a no-op: the same names are not
// duplicated and no error is returned.
func TestSeedStandardTypesCreatesAndIsIdempotent(t *testing.T) {
	db := newIssueTestDB(t)
	// issue_types.project_id is a NOT NULL FK to projects(id) in the real
	// schema, so the project the types belong to must exist first.
	createTestProject(t, db, "p1")
	svc := NewService(db)

	names := []string{"Epic", "Story", "Task", "Bug", "Subtask"}
	if err := svc.SeedStandardTypes("p1", names); err != nil {
		t.Fatalf("SeedStandardTypes: %v", err)
	}

	var count int64
	if err := db.Model(&IssueType{}).Where("project_id = ?", "p1").Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 5 {
		t.Fatalf("type count = %d, want 5", count)
	}

	var sub IssueType
	if err := db.Where("project_id = ? AND name = ?", "p1", "Subtask").First(&sub).Error; err != nil {
		t.Fatal(err)
	}
	if !sub.IsSubtask {
		t.Errorf("Subtask IsSubtask = false, want true")
	}

	var bug IssueType
	if err := db.Where("project_id = ? AND name = ?", "p1", "Bug").First(&bug).Error; err != nil {
		t.Fatal(err)
	}
	if bug.Icon != "bug" {
		t.Errorf("Bug icon = %q, want %q", bug.Icon, "bug")
	}

	// Idempotent: re-running must not duplicate and must not error.
	if err := svc.SeedStandardTypes("p1", names); err != nil {
		t.Fatalf("SeedStandardTypes (second call): %v", err)
	}
	if err := db.Model(&IssueType{}).Where("project_id = ?", "p1").Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 5 {
		t.Fatalf("after re-run type count = %d, want 5 (no duplicates)", count)
	}
}

// TestUpdateEmptyAssigneeStoresNull verifies that unassigning an issue (an empty
// accountId reaching Update) writes SQL NULL to the nullable assignee_id FK
// column, not "". Before the fix the map wrote *assigneeID ("") which is silently
// accepted on SQLite but violates the FK on Postgres (SQLSTATE 23503).
func TestUpdateEmptyAssigneeStoresNull(t *testing.T) {
	db := newIssueTestDB(t)
	createTestProject(t, db, "p1")
	// assignee_id is a real FK to users(id): assigning a literal id (rather than
	// one Update itself generated) requires that user to actually exist.
	createTestUser(t, db, "user-123")
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
	createTestProject(t, db, "p1")
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

// TestCreate_StoresValidADFDescription pins the storage invariant of
// issues.description_json: the column holds the ADF document itself, as valid
// JSON, exactly as Update stores it. The handler marshals the request's ADF
// and hands the JSON string to Create; Create must persist it unchanged — not
// wrap it in the legacy {"content":"..."} envelope. Doing that wrap with
// fmt.Sprintf produced INVALID JSON (the inner quotes were never escaped), so
// every description typed in the create modal came back as raw JSON text.
func TestCreate_StoresValidADFDescription(t *testing.T) {
	db := newIssueTestDB(t)
	createTestProject(t, db, "proj-adf")
	svc := NewService(db)

	doc := adf.Node{Type: "doc", Version: 1, Content: []adf.Node{{
		Type: "paragraph",
		Content: []adf.Node{
			{Type: "text", Text: "Hello "},
			{Type: "text", Text: "world", Marks: []adf.Mark{{Type: "strong"}}},
		},
	}}}
	in, err := json.Marshal(doc)
	if err != nil {
		t.Fatalf("marshal doc: %v", err)
	}

	iss, err := svc.Create("proj-adf", "proj-adf", "adf description", string(in), PriorityMedium, nil, nil)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	var stored string
	if err := db.Raw("SELECT description_json FROM issues WHERE id = ?", iss.ID).Scan(&stored).Error; err != nil {
		t.Fatalf("read description_json: %v", err)
	}
	if !json.Valid([]byte(stored)) {
		t.Fatalf("stored description_json is not valid JSON: %s", stored)
	}
	var got adf.Node
	if err := json.Unmarshal([]byte(stored), &got); err != nil {
		t.Fatalf("stored description_json does not unmarshal as ADF: %v (%s)", err, stored)
	}
	if !reflect.DeepEqual(got, doc) {
		t.Fatalf("stored ADF differs from input\n got: %s\nwant: %s", stored, in)
	}
	if iss.DescriptionJSON != stored {
		t.Errorf("returned DescriptionJSON %q != stored %q", iss.DescriptionJSON, stored)
	}
}

// TestCreate_PlainTextDescriptionStoredAsADF: callers that pass plain text
// (cmd/seed, programmatic use) must still work — the text is wrapped into an
// ADF doc via json.Marshal, never by string formatting, so quotes and
// backslashes in the text can't break the stored JSON.
func TestCreate_PlainTextDescriptionStoredAsADF(t *testing.T) {
	db := newIssueTestDB(t)
	createTestProject(t, db, "proj-txt")
	svc := NewService(db)

	text := `say "hi" \ back` + "\nsecond line"
	iss, err := svc.Create("proj-txt", "proj-txt", "plain description", text, PriorityMedium, nil, nil)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if !json.Valid([]byte(iss.DescriptionJSON)) {
		t.Fatalf("stored description_json is not valid JSON: %s", iss.DescriptionJSON)
	}
	var got adf.Node
	if err := json.Unmarshal([]byte(iss.DescriptionJSON), &got); err != nil || got.Type != "doc" {
		t.Fatalf("stored description is not an ADF doc: %v (%s)", err, iss.DescriptionJSON)
	}
	if pt := adf.PlainText(got); pt != text {
		t.Errorf("PlainText = %q, want %q", pt, text)
	}
}

// TestCreate_EmptyDescriptionStaysEmpty: no description → the column keeps
// its "no description" value ({} — the schema default, which the v3 mapping
// renders as a null description), not an ADF doc with an empty paragraph.
func TestCreate_EmptyDescriptionStaysEmpty(t *testing.T) {
	db := newIssueTestDB(t)
	createTestProject(t, db, "proj-empty")
	svc := NewService(db)

	iss, err := svc.Create("proj-empty", "proj-empty", "no description", "", PriorityMedium, nil, nil)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	var stored string
	if err := db.Raw("SELECT description_json FROM issues WHERE id = ?", iss.ID).Scan(&stored).Error; err != nil {
		t.Fatalf("read description_json: %v", err)
	}
	if stored != "{}" {
		t.Fatalf("stored description_json = %q, want {}", stored)
	}
}
