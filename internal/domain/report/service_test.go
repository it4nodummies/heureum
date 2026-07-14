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
	// L'issue è ATTUALMENTE in TODO: se il CFD (bug) unisse sullo status
	// corrente (i.status_id), la categoria 'done' risulterebbe vuota anche
	// se in passato l'issue è transitata per DONE. Solo la query corretta,
	// che unisce su ih.new_value (lo stato storico dell'evento), può
	// popolare 'done' qui.
	iss := &issue.Issue{ID: uuid.NewString(), ProjectID: "proj-1", Key: "P-1", Title: "x", SeqID: 1, StatusID: &todoID}
	db.Create(iss)
	// evento 'created' (entra in todo) e 'status' → done (poi tornata in todo,
	// da cui lo stato attuale diverso da quello dell'evento storico)
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

// TestCFD_CumulativeShape verifica che la CFD sia una vera cumulata giorno-per-giorno
// (l'issue migra da todo→done nel tempo), non una serie piatta col totale ripetuto.
func TestCFD_CumulativeShape(t *testing.T) {
	db := newDB(t)
	todoID, doneID := seedWorkflow(t, db, "proj-1")
	iss := &issue.Issue{ID: uuid.NewString(), ProjectID: "proj-1", Key: "P-1", Title: "x", SeqID: 1, StatusID: &todoID}
	db.Create(iss)
	// giorno -3: creata (entra in todo); giorno -1: passa a done
	db.Create(&issue.IssueHistory{ID: uuid.NewString(), IssueID: iss.ID, FieldName: "created", OldValue: "", NewValue: "P-1", CreatedAt: time.Now().AddDate(0, 0, -3)})
	db.Create(&issue.IssueHistory{ID: uuid.NewString(), IssueID: iss.ID, FieldName: "status", OldValue: todoID, NewValue: doneID, CreatedAt: time.Now().AddDate(0, 0, -1)})

	cfd, err := NewService(db).GetCFD("proj-1")
	if err != nil {
		t.Fatalf("GetCFD: %v", err)
	}
	if len(cfd.Dates) < 2 {
		t.Fatalf("attese >=2 date, %d", len(cfd.Dates))
	}
	last := len(cfd.Dates) - 1
	// primo giorno: 1 in todo, 0 in done. Ultimo giorno: 0 in todo, 1 in done.
	if cfd.Data["todo"][0] != 1 || cfd.Data["done"][0] != 0 {
		t.Errorf("primo giorno atteso todo=1 done=0, got todo=%d done=%d", cfd.Data["todo"][0], cfd.Data["done"][0])
	}
	if cfd.Data["todo"][last] != 0 || cfd.Data["done"][last] != 1 {
		t.Errorf("ultimo giorno atteso todo=0 done=1, got todo=%d done=%d", cfd.Data["todo"][last], cfd.Data["done"][last])
	}
	// la serie NON deve essere piatta (il bug assegnava lo stesso totale a ogni giorno)
	if cfd.Data["done"][0] == cfd.Data["done"][last] {
		t.Errorf("la serie done è piatta (%v) — deve crescere nel tempo", cfd.Data["done"])
	}
}

func TestPieByField_Status(t *testing.T) {
	db := newDB(t)
	todoID, doneID := seedWorkflow(t, db, "proj-1")
	db.Create(&issue.Issue{ID: uuid.NewString(), ProjectID: "proj-1", Key: "P-1", Title: "a", SeqID: 1, StatusID: &todoID})
	db.Create(&issue.Issue{ID: uuid.NewString(), ProjectID: "proj-1", Key: "P-2", Title: "b", SeqID: 2, StatusID: &doneID})
	db.Create(&issue.Issue{ID: uuid.NewString(), ProjectID: "proj-1", Key: "P-3", Title: "c", SeqID: 3, StatusID: &doneID})

	svc := NewService(db)
	slices, err := svc.GetPieByField("proj-1", "status")
	if err != nil {
		t.Fatalf("GetPieByField: %v", err)
	}
	byLabel := map[string]int{}
	for _, s := range slices {
		byLabel[s.Label] = s.Count
	}
	if byLabel["TO DO"] != 1 || byLabel["DONE"] != 2 {
		t.Errorf("conteggi torta errati: %v", byLabel)
	}
}

func TestPieByField_Priority(t *testing.T) {
	db := newDB(t)
	db.Create(&issue.Issue{ID: uuid.NewString(), ProjectID: "proj-1", Key: "P-1", Title: "a", SeqID: 1, Priority: issue.PriorityHigh})
	db.Create(&issue.Issue{ID: uuid.NewString(), ProjectID: "proj-1", Key: "P-2", Title: "b", SeqID: 2, Priority: issue.PriorityHigh})
	svc := NewService(db)
	slices, err := svc.GetPieByField("proj-1", "priority")
	if err != nil {
		t.Fatalf("GetPieByField: %v", err)
	}
	if len(slices) != 1 || slices[0].Label != "high" || slices[0].Count != 2 {
		t.Errorf("torta priority errata: %+v", slices)
	}
}

func TestPieByField_Invalid(t *testing.T) {
	svc := NewService(newDB(t))
	if _, err := svc.GetPieByField("proj-1", "bogus"); err == nil {
		t.Error("atteso errore per campo non supportato")
	}
}

func TestCreatedVsResolved(t *testing.T) {
	db := newDB(t)
	_, doneID := seedWorkflow(t, db, "proj-1")
	now := time.Now()
	// una issue creata 2 giorni fa
	c := &issue.Issue{ID: uuid.NewString(), ProjectID: "proj-1", Key: "P-1", Title: "a", SeqID: 1, StatusID: &doneID, CreatedAt: now.AddDate(0, 0, -2)}
	db.Create(c)
	db.Create(&issue.IssueHistory{ID: uuid.NewString(), IssueID: c.ID, FieldName: "created", OldValue: "", NewValue: "P-1", CreatedAt: now.AddDate(0, 0, -2)})
	// risolta (status → done) ieri
	db.Create(&issue.IssueHistory{ID: uuid.NewString(), IssueID: c.ID, FieldName: "status", OldValue: "x", NewValue: doneID, CreatedAt: now.AddDate(0, 0, -1)})

	svc := NewService(db)
	data, err := svc.GetCreatedVsResolved("proj-1", 7)
	if err != nil {
		t.Fatalf("GetCreatedVsResolved: %v", err)
	}
	if len(data.Dates) != 7 {
		t.Fatalf("attesi 7 giorni, %d", len(data.Dates))
	}
	sumC, sumR := 0, 0
	for i := range data.Dates {
		sumC += data.Created[i]
		sumR += data.Resolved[i]
	}
	if sumC != 1 {
		t.Errorf("created totali attesi 1, %d", sumC)
	}
	if sumR != 1 {
		t.Errorf("resolved totali attesi 1, %d", sumR)
	}
}
