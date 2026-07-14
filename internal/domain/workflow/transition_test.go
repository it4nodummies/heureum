package workflow

import (
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newWFDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if err := db.AutoMigrate(&Workflow{}, &WorkflowStatus{}, &WorkflowTransition{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

func TestAddTransition_WithNameAndRules(t *testing.T) {
	db := newWFDB(t)
	svc := NewService(db)
	wf, _ := svc.CreateWorkflow("proj-1", "WF")
	todo, _ := svc.AddStatus(wf.ID, "To Do", CategoryTodo, "#111")
	done, _ := svc.AddStatus(wf.ID, "Done", CategoryDone, "#222")

	tr, err := svc.AddTransition(wf.ID, todo.ID, done.ID, "Resolve", true, true)
	if err != nil {
		t.Fatalf("AddTransition: %v", err)
	}
	if tr.Name != "Resolve" || !tr.RequireAssignee || !tr.SetResolution {
		t.Errorf("campi transizione errati: %+v", tr)
	}
}

func TestGetTransitionByID(t *testing.T) {
	db := newWFDB(t)
	svc := NewService(db)
	wf, _ := svc.CreateWorkflow("proj-1", "WF")
	a, _ := svc.AddStatus(wf.ID, "A", CategoryTodo, "#111")
	b, _ := svc.AddStatus(wf.ID, "B", CategoryInProgress, "#222")
	tr, _ := svc.AddTransition(wf.ID, a.ID, b.ID, "Go", false, false)

	got, err := svc.GetTransitionByID(tr.ID)
	if err != nil {
		t.Fatalf("GetTransitionByID: %v", err)
	}
	if got.ToStatusID != b.ID {
		t.Errorf("toStatus errato: %+v", got)
	}
	if _, err := svc.GetTransitionByID("nope"); err == nil {
		t.Error("atteso errore per id inesistente")
	}
}

func TestGetAvailableTransitions(t *testing.T) {
	db := newWFDB(t)
	svc := NewService(db)
	wf, _ := svc.CreateWorkflow("proj-1", "WF")
	a, _ := svc.AddStatus(wf.ID, "A", CategoryTodo, "#111")
	b, _ := svc.AddStatus(wf.ID, "B", CategoryInProgress, "#222")
	c, _ := svc.AddStatus(wf.ID, "C", CategoryDone, "#333")
	svc.AddTransition(wf.ID, a.ID, b.ID, "A→B", false, false)
	svc.AddTransition(wf.ID, a.ID, c.ID, "A→C", false, false)
	svc.AddTransition(wf.ID, b.ID, c.ID, "B→C", false, false)

	avail, err := svc.GetAvailableTransitions(wf.ID, a.ID)
	if err != nil {
		t.Fatalf("GetAvailableTransitions: %v", err)
	}
	if len(avail) != 2 {
		t.Errorf("attese 2 transizioni da A, got %d", len(avail))
	}
}

func TestReorderStatuses(t *testing.T) {
	db := newWFDB(t)
	svc := NewService(db)
	wf, _ := svc.CreateWorkflow("proj-1", "WF")
	a, _ := svc.AddStatus(wf.ID, "A", CategoryTodo, "#111")
	b, _ := svc.AddStatus(wf.ID, "B", CategoryInProgress, "#222")
	c, _ := svc.AddStatus(wf.ID, "C", CategoryDone, "#333")

	if err := svc.ReorderStatuses(wf.ID, []string{c.ID, a.ID, b.ID}); err != nil {
		t.Fatalf("ReorderStatuses: %v", err)
	}
	wf2, _ := svc.GetWorkflow("proj-1")
	// GetWorkflow preload ordina per position? verifichiamo le position assegnate
	posByID := map[string]int{}
	for _, st := range wf2.Statuses {
		posByID[st.ID] = st.Position
	}
	if !(posByID[c.ID] < posByID[a.ID] && posByID[a.ID] < posByID[b.ID]) {
		t.Errorf("ordine posizioni errato: %v", posByID)
	}
}
