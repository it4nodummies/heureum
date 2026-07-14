package report

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/open-jira/open-jira/internal/domain/issue"
	"github.com/open-jira/open-jira/internal/domain/sprint"
	"github.com/open-jira/open-jira/internal/domain/workflow"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if err := db.AutoMigrate(&issue.Issue{}, &issue.IssueHistory{}, &sprint.Sprint{}, &workflow.Workflow{}, &workflow.WorkflowStatus{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

// seedWorkflow crea uno workflow con TO DO(todo)/DONE(done) e ritorna gli id.
func seedWorkflow(t *testing.T, db *gorm.DB, projectID string) (todoID, doneID string) {
	t.Helper()
	wf := &workflow.Workflow{ID: uuid.NewString(), ProjectID: projectID, Name: "WF"}
	db.Create(wf)
	todo := &workflow.WorkflowStatus{ID: uuid.NewString(), WorkflowID: wf.ID, Name: "TO DO", Category: workflow.CategoryTodo, Position: 0}
	done := &workflow.WorkflowStatus{ID: uuid.NewString(), WorkflowID: wf.ID, Name: "DONE", Category: workflow.CategoryDone, Position: 1}
	db.Create(todo)
	db.Create(done)
	return todo.ID, done.ID
}

func TestBurndown_ReadsStatusHistory(t *testing.T) {
	db := newDB(t)
	todoID, doneID := seedWorkflow(t, db, "proj-1")
	start := time.Now().AddDate(0, 0, -3)
	end := time.Now().AddDate(0, 0, 3)
	sp := &sprint.Sprint{ID: uuid.NewString(), ProjectID: "proj-1", Name: "S1", State: sprint.StateActive, StartDate: &start, EndDate: &end, SeqID: 1}
	db.Create(sp)
	// una issue da 5 punti, nello sprint, passata a DONE ieri.
	// CreatedAt esplicito (prima dell'inizio sprint): GetBurndownData usa
	// iss.CreatedAt per decidere se l'issue è "nello sprint" in un dato giorno,
	// e GORM autoCreateTime la sovrascriverebbe con "adesso" se lasciata a zero.
	iss := &issue.Issue{ID: uuid.NewString(), ProjectID: "proj-1", Key: "P-1", Title: "x", SeqID: 1, StoryPoints: 5, SprintID: &sp.ID, StatusID: &doneID, CreatedAt: start}
	db.Create(iss)
	db.Create(&issue.IssueHistory{ID: uuid.NewString(), IssueID: iss.ID, FieldName: "story_points", OldValue: "0", NewValue: "5", CreatedAt: start})
	db.Create(&issue.IssueHistory{ID: uuid.NewString(), IssueID: iss.ID, FieldName: "status", OldValue: todoID, NewValue: doneID, CreatedAt: time.Now().AddDate(0, 0, -1)})

	svc := NewService(db)
	data, err := svc.GetBurndownData(sp.ID)
	if err != nil {
		t.Fatalf("GetBurndownData: %v", err)
	}
	if len(data.Actual) == 0 {
		t.Fatal("Actual vuoto")
	}
	// dopo il passaggio a DONE il lavoro rimanente cala: l'ultimo valore < primo
	if data.Actual[len(data.Actual)-1] >= data.Actual[0] {
		t.Errorf("il burndown deve scendere dopo il completamento: %v", data.Actual)
	}
}

func TestVelocity_ClosedSprints(t *testing.T) {
	db := newDB(t)
	_, doneID := seedWorkflow(t, db, "proj-1")
	cd := time.Now().AddDate(0, 0, -1)
	sp := &sprint.Sprint{ID: uuid.NewString(), ProjectID: "proj-1", Name: "S1", State: sprint.StateClosed, CompleteDate: &cd, SeqID: 1}
	db.Create(sp)
	iss := &issue.Issue{ID: uuid.NewString(), ProjectID: "proj-1", Key: "P-1", Title: "x", SeqID: 1, StoryPoints: 8, SprintID: &sp.ID, StatusID: &doneID}
	db.Create(iss)

	svc := NewService(db)
	v, err := svc.GetVelocity("proj-1")
	if err != nil {
		t.Fatalf("GetVelocity: %v", err)
	}
	if len(v.Sprints) != 1 {
		t.Fatalf("atteso 1 sprint chiuso, %d", len(v.Sprints))
	}
	if v.Sprints[0].Completed != 8 {
		t.Errorf("completed atteso 8, got %d", v.Sprints[0].Completed)
	}
}

func TestProjectSummary_Counts(t *testing.T) {
	db := newDB(t)
	todoID, doneID := seedWorkflow(t, db, "proj-1")
	db.Create(&issue.Issue{ID: uuid.NewString(), ProjectID: "proj-1", Key: "P-1", Title: "a", SeqID: 1, StatusID: &todoID})
	db.Create(&issue.Issue{ID: uuid.NewString(), ProjectID: "proj-1", Key: "P-2", Title: "b", SeqID: 2, StatusID: &doneID})

	svc := NewService(db)
	sum, err := svc.GetProjectSummary("proj-1")
	if err != nil {
		t.Fatalf("GetProjectSummary: %v", err)
	}
	total := 0
	for _, n := range sum.IssueCountByStatus {
		total += n
	}
	if total != 2 {
		t.Errorf("attese 2 issue nei conteggi per stato, got %d (%v)", total, sum.IssueCountByStatus)
	}
}

func TestCFD_ReadsHistoricalStatus(t *testing.T) {
	db := newDB(t)
	todoID, doneID := seedWorkflow(t, db, "proj-1")
	iss := &issue.Issue{ID: uuid.NewString(), ProjectID: "proj-1", Key: "P-1", Title: "x", SeqID: 1, StatusID: &doneID}
	db.Create(iss)
	// evento 'created' (entra in todo) e 'status' → done
	db.Create(&issue.IssueHistory{ID: uuid.NewString(), IssueID: iss.ID, FieldName: "created", OldValue: "", NewValue: "P-1", CreatedAt: time.Now().AddDate(0, 0, -2)})
	db.Create(&issue.IssueHistory{ID: uuid.NewString(), IssueID: iss.ID, FieldName: "status", OldValue: todoID, NewValue: doneID, CreatedAt: time.Now().AddDate(0, 0, -1)})

	svc := NewService(db)
	cfd, err := svc.GetCFD("proj-1")
	if err != nil {
		t.Fatalf("GetCFD: %v", err)
	}
	if len(cfd.Dates) == 0 {
		t.Fatal("CFD senza date")
	}
	// deve esistere un conteggio non-zero per la categoria 'done' (dallo status storico)
	done, ok := cfd.Data["done"]
	if !ok {
		t.Fatalf("categoria 'done' assente nel CFD: chiavi %v", keysOf(cfd.Data))
	}
	sum := 0
	for _, n := range done {
		sum += n
	}
	if sum == 0 {
		t.Errorf("la categoria done deve avere conteggi > 0 dallo status storico")
	}
}

func keysOf(m map[string][]int) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
