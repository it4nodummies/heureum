package sprint

import (
	"errors"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/it4nodummies/heureum/internal/storetest"
)

func newDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if err := db.AutoMigrate(&Sprint{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

func TestCreateFull_AssignsSeqIDAndFields(t *testing.T) {
	svc := NewService(newDB(t))
	boardID := int64(7)
	start := time.Date(2026, 7, 14, 9, 0, 0, 0, time.UTC)
	end := time.Date(2026, 7, 28, 17, 0, 0, 0, time.UTC)
	sp, err := svc.CreateFull("proj-1", "Sprint 1", "ship it", &boardID, &start, &end)
	if err != nil {
		t.Fatalf("CreateFull: %v", err)
	}
	if sp.SeqID != 1 {
		t.Errorf("seq_id atteso 1, got %d", sp.SeqID)
	}
	if sp.OriginBoardID == nil || *sp.OriginBoardID != 7 {
		t.Errorf("originBoardID errato: %v", sp.OriginBoardID)
	}
	if sp.StartDate == nil || !sp.StartDate.Equal(start) {
		t.Errorf("startDate errata")
	}
	if sp.State != StateFuture {
		t.Errorf("stato iniziale atteso future, got %s", sp.State)
	}
}

func TestGetBySeqID(t *testing.T) {
	svc := NewService(newDB(t))
	sp, _ := svc.CreateFull("proj-1", "S1", "", nil, nil, nil)
	got, err := svc.GetBySeqID(sp.SeqID)
	if err != nil {
		t.Fatalf("GetBySeqID: %v", err)
	}
	if got.ID != sp.ID {
		t.Error("id mismatch")
	}
}

func TestComplete_SetsCompleteDate(t *testing.T) {
	svc := NewService(newDB(t))
	sp, _ := svc.CreateFull("proj-1", "S1", "", nil, nil, nil)
	svc.Start(sp.ProjectID, sp.ID)
	done, err := svc.Complete(sp.ProjectID, sp.ID, false, nil)
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if done.State != StateClosed {
		t.Errorf("stato atteso closed, got %s", done.State)
	}
	if done.CompleteDate == nil {
		t.Error("completeDate deve essere valorizzata dopo Complete")
	}
}

// seedIssuesTables crea le tabelle minime toccate dalla query di Complete
// (issues + workflow_statuses) senza importare il dominio issue (evita cicli).
func seedIssuesTables(t *testing.T, db *gorm.DB) {
	t.Helper()
	if err := db.Exec("CREATE TABLE workflow_statuses (id TEXT PRIMARY KEY, category TEXT)").Error; err != nil {
		t.Fatalf("create workflow_statuses: %v", err)
	}
	if err := db.Exec("CREATE TABLE issues (id TEXT PRIMARY KEY, sprint_id TEXT, status_id TEXT)").Error; err != nil {
		t.Fatalf("create issues: %v", err)
	}
	db.Exec("INSERT INTO workflow_statuses (id, category) VALUES ('st-todo','todo'), ('st-done','done')")
}

func TestComplete_MoveIncompleteToAnotherSprint(t *testing.T) {
	db := newDB(t)
	svc := NewService(db)
	seedIssuesTables(t, db)

	s1, _ := svc.CreateFull("proj-1", "S1", "", nil, nil, nil)
	s2, _ := svc.CreateFull("proj-1", "S2", "", nil, nil, nil)
	svc.Start(s1.ProjectID, s1.ID)

	// una issue incompleta (todo) e una completata (done) su s1
	db.Exec("INSERT INTO issues (id, sprint_id, status_id) VALUES ('iss-open', ?, 'st-todo')", s1.ID)
	db.Exec("INSERT INTO issues (id, sprint_id, status_id) VALUES ('iss-done', ?, 'st-done')", s1.ID)

	done, err := svc.Complete(s1.ProjectID, s1.ID, false, &s2.ID)
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if done.State != StateClosed {
		t.Errorf("s1 atteso closed, got %s", done.State)
	}

	var openSprint, doneSprint string
	db.Raw("SELECT sprint_id FROM issues WHERE id = 'iss-open'").Scan(&openSprint)
	db.Raw("SELECT sprint_id FROM issues WHERE id = 'iss-done'").Scan(&doneSprint)
	if openSprint != s2.ID {
		t.Errorf("issue incompleta attesa su s2 (%s), got %q", s2.ID, openSprint)
	}
	if doneSprint != s1.ID {
		t.Errorf("issue completata deve restare su s1 (%s), got %q", s1.ID, doneSprint)
	}
}

func TestComplete_MoveIncompleteToBacklog(t *testing.T) {
	db := newDB(t)
	svc := NewService(db)
	seedIssuesTables(t, db)

	s3, _ := svc.CreateFull("proj-1", "S3", "", nil, nil, nil)
	svc.Start(s3.ProjectID, s3.ID)
	db.Exec("INSERT INTO issues (id, sprint_id, status_id) VALUES ('iss-open2', ?, 'st-todo')", s3.ID)
	db.Exec("INSERT INTO issues (id, sprint_id, status_id) VALUES ('iss-done2', ?, 'st-done')", s3.ID)

	if _, err := svc.Complete(s3.ProjectID, s3.ID, true, nil); err != nil {
		t.Fatalf("Complete: %v", err)
	}

	var openSprint *string
	var doneSprint string
	db.Raw("SELECT sprint_id FROM issues WHERE id = 'iss-open2'").Scan(&openSprint)
	db.Raw("SELECT sprint_id FROM issues WHERE id = 'iss-done2'").Scan(&doneSprint)
	if openSprint != nil {
		t.Errorf("issue incompleta attesa in backlog (NULL), got %q", *openSprint)
	}
	if doneSprint != s3.ID {
		t.Errorf("issue completata deve restare su s3 (%s), got %q", s3.ID, doneSprint)
	}
}

// TestProjectScopedOps_PropagateDBErrors: only gorm.ErrRecordNotFound maps to
// ErrNotFound; a real DB failure (here: the table is gone) must propagate so
// the handler answers 500 instead of a misleading 404.
func TestProjectScopedOps_PropagateDBErrors(t *testing.T) {
	db := newDB(t)
	svc := NewService(db)
	if err := db.Exec("DROP TABLE sprints").Error; err != nil {
		t.Fatal(err)
	}
	check := func(what string, err error) {
		t.Helper()
		if err == nil {
			t.Errorf("%s: err = nil, want a DB error", what)
			return
		}
		if errors.Is(err, ErrNotFound) {
			t.Errorf("%s: DB error collapsed into ErrNotFound: %v", what, err)
		}
	}
	_, err := svc.GetByID("p", "s")
	check("GetByID", err)
	_, err = svc.Update("p", "s", "n", "g")
	check("Update", err)
	_, err = svc.Start("p", "s")
	check("Start", err)
	_, err = svc.Complete("p", "s", true, nil)
	check("Complete", err)

	// And a plain miss is still ErrNotFound.
	db2 := newDB(t)
	if _, err := NewService(db2).GetByID("p", "missing"); !errors.Is(err, ErrNotFound) {
		t.Errorf("GetByID(missing) = %v, want ErrNotFound", err)
	}
}

// --- Real-schema tests (storetest): FKs enforced, real column types. ---

// seedRealSprintProject inserts the project, workflow and the two statuses
// (todo/done) the real-schema Complete tests need: sprints.project_id,
// issues.project_id and issues.status_id are enforced FKs here.
func seedRealSprintProject(t *testing.T, db *gorm.DB) {
	t.Helper()
	for _, stmt := range []string{
		"INSERT INTO projects (id, name, key) VALUES ('p1', 'P1', 'P1')",
		"INSERT INTO workflows (id, project_id, name) VALUES ('wf1', 'p1', 'WF')",
		"INSERT INTO workflow_statuses (id, workflow_id, name, category) VALUES ('st-todo', 'wf1', 'To Do', 'todo'), ('st-done', 'wf1', 'Done', 'done')",
	} {
		if err := db.Exec(stmt).Error; err != nil {
			t.Fatalf("seed: %v", err)
		}
	}
}

var realIssueSeq int64 = 70000

// insertRealIssue inserts a minimal valid issues row on sprint sprintID with
// the given status (nil → status_id IS NULL, the case Complete used to skip).
func insertRealIssue(t *testing.T, db *gorm.DB, id, sprintID string, statusID *string) {
	t.Helper()
	realIssueSeq++
	if err := db.Exec(
		"INSERT INTO issues (id, project_id, key, title, seq_id, sprint_id, status_id) VALUES (?, 'p1', ?, ?, ?, ?, ?)",
		id, id, id, realIssueSeq, sprintID, statusID,
	).Error; err != nil {
		t.Fatalf("insert issue %s: %v", id, err)
	}
}

func sprintOf(t *testing.T, db *gorm.DB, issueID string) *string {
	t.Helper()
	var sprintID *string
	if err := db.Raw("SELECT sprint_id FROM issues WHERE id = ?", issueID).Scan(&sprintID).Error; err != nil {
		t.Fatalf("read sprint_id of %s: %v", issueID, err)
	}
	return sprintID
}

func strp(s string) *string { return &s }

// TestComplete_MovesNullStatusIssuesToBacklog: an issue whose status_id is
// NULL is open, not done — it must leave the closed sprint like any other
// open issue. `status_id NOT IN (done)` is NULL-unsafe and silently kept it.
func TestComplete_MovesNullStatusIssuesToBacklog(t *testing.T) {
	db := storetest.New(t)
	seedRealSprintProject(t, db)
	svc := NewService(db)
	s1, err := svc.CreateFull("p1", "S1", "", nil, nil, nil)
	if err != nil {
		t.Fatalf("CreateFull: %v", err)
	}
	if _, err := svc.Start("p1", s1.ID); err != nil {
		t.Fatalf("Start: %v", err)
	}
	insertRealIssue(t, db, "iss-null", s1.ID, nil)
	insertRealIssue(t, db, "iss-todo", s1.ID, strp("st-todo"))
	insertRealIssue(t, db, "iss-done", s1.ID, strp("st-done"))

	if _, err := svc.Complete("p1", s1.ID, true, nil); err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if got := sprintOf(t, db, "iss-null"); got != nil {
		t.Errorf("NULL-status issue still on the closed sprint (%q), want backlog", *got)
	}
	if got := sprintOf(t, db, "iss-todo"); got != nil {
		t.Errorf("todo issue still on the closed sprint (%q), want backlog", *got)
	}
	if got := sprintOf(t, db, "iss-done"); got == nil || *got != s1.ID {
		t.Errorf("done issue must stay on the closed sprint, got %v", got)
	}
}

// TestComplete_MovesNullStatusIssuesToNextSprint: same NULL-status case for
// the move-to-sprint branch.
func TestComplete_MovesNullStatusIssuesToNextSprint(t *testing.T) {
	db := storetest.New(t)
	seedRealSprintProject(t, db)
	svc := NewService(db)
	s1, _ := svc.CreateFull("p1", "S1", "", nil, nil, nil)
	s2, _ := svc.CreateFull("p1", "S2", "", nil, nil, nil)
	if _, err := svc.Start("p1", s1.ID); err != nil {
		t.Fatalf("Start: %v", err)
	}
	insertRealIssue(t, db, "iss-null", s1.ID, nil)
	insertRealIssue(t, db, "iss-todo", s1.ID, strp("st-todo"))
	insertRealIssue(t, db, "iss-done", s1.ID, strp("st-done"))

	if _, err := svc.Complete("p1", s1.ID, false, &s2.ID); err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if got := sprintOf(t, db, "iss-null"); got == nil || *got != s2.ID {
		t.Errorf("NULL-status issue: sprint = %v, want %s", got, s2.ID)
	}
	if got := sprintOf(t, db, "iss-todo"); got == nil || *got != s2.ID {
		t.Errorf("todo issue: sprint = %v, want %s", got, s2.ID)
	}
	if got := sprintOf(t, db, "iss-done"); got == nil || *got != s1.ID {
		t.Errorf("done issue must stay on the closed sprint, got %v", got)
	}
}

// TestComplete_RollsBackOnMoveFailure: the state change and the issue moves
// are one transaction. If moving the issues fails (here: the issues table is
// gone), Complete must return the error and the sprint must still be active —
// not closed with its open issues stranded on it.
func TestComplete_RollsBackOnMoveFailure(t *testing.T) {
	db := storetest.New(t)
	seedRealSprintProject(t, db)
	svc := NewService(db)
	s1, _ := svc.CreateFull("p1", "S1", "", nil, nil, nil)
	if _, err := svc.Start("p1", s1.ID); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if err := db.Exec("ALTER TABLE issues RENAME TO issues_gone").Error; err != nil {
		t.Fatalf("rename issues: %v", err)
	}

	if _, err := svc.Complete("p1", s1.ID, true, nil); err == nil {
		t.Fatal("Complete: err = nil, want the failed issue move to be reported")
	}
	got, err := svc.GetByID("p1", s1.ID)
	if err != nil {
		t.Fatalf("reload sprint: %v", err)
	}
	if got.State != StateActive {
		t.Errorf("sprint state = %q after a failed completion, want %q (rolled back)", got.State, StateActive)
	}
	if got.CompleteDate != nil {
		t.Errorf("complete_date set (%v) although the completion failed", got.CompleteDate)
	}
}
