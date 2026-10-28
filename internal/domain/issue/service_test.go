package issue

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"sync"
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

// insertRawIssueKey mimics a fixture (or a pre-counter row) that writes an
// issue with a hand-picked key, bypassing Service.Create and therefore the
// per-project counter.
func insertRawIssueKey(t *testing.T, db *gorm.DB, projectID, key string) {
	t.Helper()
	if err := db.Create(&Issue{
		ID:        "raw-" + key,
		ProjectID: projectID,
		Key:       key,
		Title:     key,
		SeqID:     nextTestSeqID(),
	}).Error; err != nil {
		t.Fatalf("insertRawIssueKey(%q): %v", key, err)
	}
}

func readIssueCounter(t *testing.T, db *gorm.DB, projectID string) int64 {
	t.Helper()
	var n int64
	if err := db.Raw("SELECT issue_counter FROM projects WHERE id = ?", projectID).Scan(&n).Error; err != nil {
		t.Fatalf("read issue_counter: %v", err)
	}
	return n
}

// TestCreate_AssignsSequentialKeysAtomically pins the per-project counter:
// N concurrent creates in one project yield exactly the keys P-1..P-N, N
// distinct seq_ids, and leave projects.issue_counter == N. On SQLite the
// single-writer lock serializes the transactions, so the race proper is
// only exercised on the Postgres CI leg; the counter column and its final
// value are asserted on every driver so the test is RED without the
// migration regardless of scheduling luck.
func TestCreate_AssignsSequentialKeysAtomically(t *testing.T) {
	db := newIssueTestDB(t)
	createTestProject(t, db, "p-atomic")
	if !db.Migrator().HasColumn("projects", "issue_counter") {
		t.Fatal("projects.issue_counter missing: migration 000026 not applied")
	}
	svc := NewService(db)

	const n = 20
	keys := make(chan string, n)
	seqIDs := make(chan int64, n)
	errs := make(chan error, n)
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			iss, err := svc.Create("P", "p-atomic", fmt.Sprintf("issue %d", i), "", PriorityMedium, nil, nil)
			if err != nil {
				errs <- err
				return
			}
			keys <- iss.Key
			seqIDs <- iss.SeqID
		}(i)
	}
	wg.Wait()
	close(keys)
	close(seqIDs)
	close(errs)
	for err := range errs {
		t.Errorf("Create: %v", err)
	}

	seenKeys := map[string]bool{}
	for k := range keys {
		if seenKeys[k] {
			t.Errorf("duplicate key %q", k)
		}
		seenKeys[k] = true
	}
	for i := 1; i <= n; i++ {
		if k := fmt.Sprintf("P-%d", i); !seenKeys[k] {
			t.Errorf("key %q never allocated (got %v)", k, seenKeys)
		}
	}
	seenSeq := map[int64]bool{}
	for s := range seqIDs {
		if seenSeq[s] {
			t.Errorf("duplicate seq_id %d", s)
		}
		seenSeq[s] = true
	}
	if len(seenSeq) != n {
		t.Errorf("distinct seq_ids = %d, want %d", len(seenSeq), n)
	}
	if got := readIssueCounter(t, db, "p-atomic"); got != n {
		t.Errorf("projects.issue_counter = %d, want %d", got, n)
	}
	var rows int64
	if err := db.Model(&Issue{}).Where("project_id = ?", "p-atomic").Count(&rows).Error; err != nil {
		t.Fatal(err)
	}
	if rows != n {
		t.Errorf("issue rows = %d, want %d", rows, n)
	}
}

// TestCreate_PropagatesCounterError: a key cannot be allocated for a project
// that does not exist. Create must fail with the domain sentinel (not raw DB
// text from a later FK violation) and must not leave any issue row behind.
func TestCreate_PropagatesCounterError(t *testing.T) {
	db := newIssueTestDB(t)
	svc := NewService(db)

	iss, err := svc.Create("NOPE", "no-such-project", "orphan", "", PriorityMedium, nil, nil)
	if err == nil {
		t.Fatalf("Create for a missing project succeeded: %+v", iss)
	}
	if !errors.Is(err, ErrProjectNotFound) {
		t.Errorf("err = %v, want ErrProjectNotFound", err)
	}
	var rows int64
	if err := db.Model(&Issue{}).Count(&rows).Error; err != nil {
		t.Fatal(err)
	}
	if rows != 0 {
		t.Errorf("issue rows after failed Create = %d, want 0 (transaction must roll back)", rows)
	}
}

// TestCreate_RollsBackCounterWhenInsertFails: the counter increment and the
// issue insert live in ONE transaction, so a failed insert (here: a parent_id
// that violates the FK to issues(id)) must leave the counter untouched and
// the next successful create must reuse the number.
func TestCreate_RollsBackCounterWhenInsertFails(t *testing.T) {
	db := newIssueTestDB(t)
	createTestProject(t, db, "p-rb")
	svc := NewService(db)

	bogusParent := "no-such-issue"
	if _, err := svc.Create("RB", "p-rb", "child of nothing", "", PriorityMedium, &bogusParent, nil); err == nil {
		t.Fatal("Create with a dangling parent_id succeeded; the real schema enforces the FK")
	}
	if got := readIssueCounter(t, db, "p-rb"); got != 0 {
		t.Errorf("issue_counter after rolled-back create = %d, want 0", got)
	}
	iss, err := svc.Create("RB", "p-rb", "first real one", "", PriorityMedium, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if iss.Key != "RB-1" {
		t.Errorf("key = %q, want RB-1 (number from the rolled-back attempt must be reused)", iss.Key)
	}
}

// TestCreate_ResyncsCounterAfterRawInserts: rows written without going
// through Create (fixtures, cmd/seed-style raw inserts, pre-000026 data not
// yet backfilled) leave the counter behind the real max key number. Create
// must notice the collision inside its transaction, re-sync the counter from
// the existing keys and still hand out a unique key — never a duplicate.
func TestCreate_ResyncsCounterAfterRawInserts(t *testing.T) {
	db := newIssueTestDB(t)
	createTestProject(t, db, "p-raw")
	for _, k := range []string{"RAW-1", "RAW-2", "RAW-7"} {
		insertRawIssueKey(t, db, "p-raw", k)
	}
	// A key with a non-numeric suffix must be ignored by the re-sync.
	insertRawIssueKey(t, db, "p-raw", "RAW-legacy")
	if got := readIssueCounter(t, db, "p-raw"); got != 0 {
		t.Fatalf("precondition: counter = %d, want 0", got)
	}
	svc := NewService(db)

	iss, err := svc.Create("RAW", "p-raw", "after raw rows", "", PriorityMedium, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if iss.Key != "RAW-8" {
		t.Errorf("key = %q, want RAW-8", iss.Key)
	}
	if got := readIssueCounter(t, db, "p-raw"); got != 8 {
		t.Errorf("issue_counter = %d, want 8", got)
	}
	next, err := svc.Create("RAW", "p-raw", "and one more", "", PriorityMedium, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if next.Key != "RAW-9" {
		t.Errorf("second key = %q, want RAW-9", next.Key)
	}
}

// TestBackfillCounters_FromExistingKeys: the startup backfill (cmd/server)
// sets issue_counter from the highest numeric key suffix of each project
// whose counter is still 0, skips projects without issues, is idempotent and
// never lowers a counter that is already set.
func TestBackfillCounters_FromExistingKeys(t *testing.T) {
	db := newIssueTestDB(t)
	// The backfill derives each project's key prefix from projects.key (unlike
	// Create, which receives it), so these projects need their real keys.
	for _, p := range [][2]string{{"p-bf", "BF"}, {"p-empty", "EMPTY"}, {"p-set", "SET"}} {
		if err := db.Exec(`INSERT INTO projects (id, name, key) VALUES (?, ?, ?)`, p[0], p[0], p[1]).Error; err != nil {
			t.Fatalf("insert project %s: %v", p[0], err)
		}
	}
	for _, k := range []string{"BF-1", "BF-12", "BF-3"} {
		insertRawIssueKey(t, db, "p-bf", k)
	}
	insertRawIssueKey(t, db, "p-set", "SET-2")
	if err := db.Exec("UPDATE projects SET issue_counter = 40 WHERE id = ?", "p-set").Error; err != nil {
		t.Fatal(err)
	}
	svc := NewService(db)

	updated, err := svc.BackfillCounters()
	if err != nil {
		t.Fatalf("BackfillCounters: %v", err)
	}
	if updated != 1 {
		t.Errorf("updated projects = %d, want 1", updated)
	}
	if got := readIssueCounter(t, db, "p-bf"); got != 12 {
		t.Errorf("p-bf counter = %d, want 12", got)
	}
	if got := readIssueCounter(t, db, "p-empty"); got != 0 {
		t.Errorf("p-empty counter = %d, want 0", got)
	}
	if got := readIssueCounter(t, db, "p-set"); got != 40 {
		t.Errorf("p-set counter = %d, want 40 (must not be lowered)", got)
	}

	again, err := svc.BackfillCounters()
	if err != nil {
		t.Fatalf("second BackfillCounters: %v", err)
	}
	if again != 0 {
		t.Errorf("second run updated %d projects, want 0 (idempotent)", again)
	}
	iss, err := svc.Create("BF", "p-bf", "after backfill", "", PriorityMedium, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if iss.Key != "BF-13" {
		t.Errorf("key after backfill = %q, want BF-13", iss.Key)
	}
}

// TestCreateWithOptions_RollsBackOnExtraFailure: a failure in the in-tx
// follow-up hook rolls back the issue row AND the key counter, and the error
// is returned (no issue_created event, no history row).
func TestCreateWithOptions_RollsBackOnExtraFailure(t *testing.T) {
	db := newIssueTestDB(t)
	createTestProject(t, db, "p1")
	svc := NewService(db)
	boom := errors.New("fix versions exploded")
	hookRan := false
	_, err := svc.CreateWithOptions("P1", "p1", "doomed", "", PriorityMedium, nil, nil, CreateOptions{
		Labels: []string{"backend"},
		Extra: func(tx *gorm.DB, iss *Issue) error {
			hookRan = true
			// The row IS visible inside the transaction...
			var n int64
			tx.Model(&Issue{}).Where("id = ?", iss.ID).Count(&n)
			if n != 1 {
				t.Errorf("issue row not visible inside the create transaction")
			}
			return boom
		},
	})
	if !errors.Is(err, boom) {
		t.Fatalf("CreateWithOptions err = %v, want the hook's error", err)
	}
	if !hookRan {
		t.Fatal("Extra hook never ran")
	}
	// ...and gone after the rollback, together with its label link, the
	// counter increment and any history.
	var issues, links, history int64
	db.Model(&Issue{}).Where("project_id = ?", "p1").Count(&issues)
	db.Model(&IssueLabel{}).Count(&links)
	db.Model(&IssueHistory{}).Count(&history)
	if issues != 0 || links != 0 || history != 0 {
		t.Errorf("after rollback: issues=%d issue_labels=%d history=%d, want all 0", issues, links, history)
	}
	var counter int64
	db.Raw("SELECT issue_counter FROM projects WHERE id = ?", "p1").Scan(&counter)
	if counter != 0 {
		t.Errorf("issue_counter = %d after a rolled-back create, want 0", counter)
	}
	// The next create still gets the first key.
	iss, err := svc.Create("P1", "p1", "ok", "", PriorityMedium, nil, nil)
	if err != nil || iss.Key != "P1-1" {
		t.Errorf("next create: key = %v err = %v, want P1-1", iss, err)
	}
}

// TestCreateWithOptions_AppliesFieldsInOneTransaction: status, assignee and
// (deduplicated) labels are on the row returned and stored; the assignee is
// notified once; only the "created" history entry exists (initial values are
// not "changes").
func TestCreateWithOptions_AppliesFieldsInOneTransaction(t *testing.T) {
	db := newIssueTestDB(t)
	createTestProject(t, db, "p1")
	createTestUser(t, db, "u1")
	if err := db.Exec("INSERT INTO workflows (id, project_id, name) VALUES ('wf1', 'p1', 'WF')").Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("INSERT INTO workflow_statuses (id, workflow_id, name, category) VALUES ('st-todo', 'wf1', 'To Do', 'todo')").Error; err != nil {
		t.Fatal(err)
	}
	svc := NewService(db)
	n := &recordingNotifier{}
	svc.SetNotifier(n)
	status, assignee := "st-todo", "u1"
	iss, err := svc.CreateWithOptions("P1", "p1", "full", "", PriorityHigh, nil, nil, CreateOptions{
		StatusID: &status, AssigneeID: &assignee, Labels: []string{"a", "", "b", "a"},
	})
	if err != nil {
		t.Fatalf("CreateWithOptions: %v", err)
	}
	stored, err := svc.GetByKey(iss.Key)
	if err != nil {
		t.Fatal(err)
	}
	if stored.StatusID == nil || *stored.StatusID != status || stored.AssigneeID == nil || *stored.AssigneeID != assignee {
		t.Errorf("stored status/assignee = %v/%v, want %s/%s", stored.StatusID, stored.AssigneeID, status, assignee)
	}
	labels, _ := svc.GetLabels(iss.ID)
	if len(labels) != 2 {
		t.Errorf("labels = %v, want 2 distinct", labels)
	}
	if n.assigned != 1 {
		t.Errorf("NotifyIssueAssigned called %d times, want 1", n.assigned)
	}
	hist, _ := svc.GetHistory(iss.ID)
	if len(hist) != 1 || hist[0].FieldName != "created" {
		t.Errorf("history = %+v, want exactly the \"created\" entry", hist)
	}
}

// TestCreateWithOptions_UnknownAssigneeIsError: an assignee that is not a
// user is ErrAssigneeNotFound and nothing is created (no FK error, no row).
func TestCreateWithOptions_UnknownAssigneeIsError(t *testing.T) {
	db := newIssueTestDB(t)
	createTestProject(t, db, "p1")
	svc := NewService(db)
	ghost := "ghost"
	_, err := svc.CreateWithOptions("P1", "p1", "x", "", PriorityMedium, nil, nil, CreateOptions{AssigneeID: &ghost})
	if !errors.Is(err, ErrAssigneeNotFound) {
		t.Fatalf("err = %v, want ErrAssigneeNotFound", err)
	}
	var issues int64
	db.Model(&Issue{}).Count(&issues)
	if issues != 0 {
		t.Errorf("%d issue rows created with an unknown assignee, want 0", issues)
	}
}

type recordingNotifier struct{ assigned, status int }

func (r *recordingNotifier) NotifyIssueAssigned(string, string, string, string) error {
	r.assigned++
	return nil
}
func (r *recordingNotifier) NotifyStatusChanged(string, string, string, string, string) error {
	r.status++
	return nil
}

// TestCreate_IssueTypeOfOtherProjectIsError: the issue type must exist AND
// belong to the project the issue is created in; a type of another project
// (or no type at all) is ErrIssueTypeNotFound and nothing is created.
func TestCreate_IssueTypeOfOtherProjectIsError(t *testing.T) {
	db := newIssueTestDB(t)
	createTestProject(t, db, "p1")
	createTestProject(t, db, "p2")
	svc := NewService(db)
	foreign, err := svc.TypeIDByName("p2", "Bug")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Create("P1", "p1", "x", "", PriorityMedium, nil, &foreign); !errors.Is(err, ErrIssueTypeNotFound) {
		t.Errorf("Create(type of p2 in p1) = %v, want ErrIssueTypeNotFound", err)
	}
	ghost := "no-such-type"
	if _, err := svc.Create("P1", "p1", "x", "", PriorityMedium, nil, &ghost); !errors.Is(err, ErrIssueTypeNotFound) {
		t.Errorf("Create(unknown type) = %v, want ErrIssueTypeNotFound", err)
	}
	var issues, counter int64
	db.Model(&Issue{}).Count(&issues)
	db.Raw("SELECT issue_counter FROM projects WHERE id = 'p1'").Scan(&counter)
	if issues != 0 || counter != 0 {
		t.Errorf("issues=%d counter=%d after rejected creates, want 0/0", issues, counter)
	}
	// The project's own type is fine.
	own, _ := svc.TypeIDByName("p1", "Bug")
	if _, err := svc.Create("P1", "p1", "ok", "", PriorityMedium, nil, &own); err != nil {
		t.Errorf("Create with the project's own type: %v", err)
	}
}
