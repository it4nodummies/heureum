package workflow

import (
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupWorkflowTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file::memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}
	db.AutoMigrate(&Workflow{}, &WorkflowStatus{}, &WorkflowTransition{})
	return db
}

func TestCreateWorkflow(t *testing.T) {
	db := setupWorkflowTestDB(t)
	svc := NewService(db)

	wf, err := svc.CreateWorkflow("proj-1", "Default Workflow")
	if err != nil {
		t.Fatalf("CreateWorkflow() error = %v", err)
	}
	if wf.ProjectID != "proj-1" {
		t.Errorf("ProjectID = %s", wf.ProjectID)
	}
	if wf.Name != "Default Workflow" {
		t.Errorf("Name = %s", wf.Name)
	}
}

func TestCreateDefaultWorkflowCreatesStatusesAndTransitions(t *testing.T) {
	db := setupWorkflowTestDB(t)
	svc := NewService(db)

	wf, err := svc.CreateDefaultWorkflow("proj-2", false)
	if err != nil {
		t.Fatalf("CreateDefaultWorkflow() error = %v", err)
	}
	if len(wf.Statuses) != 3 {
		t.Fatalf("expected 3 statuses, got %d", len(wf.Statuses))
	}

	categories := map[StatusCategory]bool{}
	names := map[string]int{}
	for _, s := range wf.Statuses {
		categories[s.Category] = true
		names[s.Name] = s.Position
	}
	if !categories[CategoryTodo] || !categories[CategoryInProgress] || !categories[CategoryDone] {
		t.Error("missing expected category")
	}
	if names["TO DO"] != 0 || names["IN PROGRESS"] != 1 || names["DONE"] != 2 {
		t.Errorf("unexpected status names/positions: %v", names)
	}

	transitions, err := svc.GetTransitions(wf.ID)
	if err != nil {
		t.Fatalf("GetTransitions() error = %v", err)
	}
	if len(transitions) < 3 {
		t.Fatalf("expected at least 3 transitions, got %d", len(transitions))
	}
}

// transitionEdge is a (name, from-name, to-name) tuple used to assert transitions
// independently of the generated status/transition UUIDs.
type transitionEdge struct {
	name string
	from string
	to   string
}

func collectTransitionEdges(t *testing.T, svc *Service, wf *Workflow) map[transitionEdge]bool {
	t.Helper()
	idToName := map[string]string{}
	for _, s := range wf.Statuses {
		idToName[s.ID] = s.Name
	}
	transitions, err := svc.GetTransitions(wf.ID)
	if err != nil {
		t.Fatalf("GetTransitions() error = %v", err)
	}
	edges := map[transitionEdge]bool{}
	for _, tr := range transitions {
		edges[transitionEdge{name: tr.Name, from: idToName[tr.FromStatusID], to: idToName[tr.ToStatusID]}] = true
	}
	return edges
}

func TestCreateDefaultWorkflow_WithReview_HasFourStatesAndTransitions(t *testing.T) {
	db := setupWorkflowTestDB(t)
	svc := NewService(db)

	wf, err := svc.CreateDefaultWorkflow("proj-review", true)
	if err != nil {
		t.Fatalf("CreateDefaultWorkflow() error = %v", err)
	}

	gotNames := make([]string, 0, len(wf.Statuses))
	for _, s := range wf.Statuses {
		gotNames = append(gotNames, s.Name)
	}
	wantNames := []string{"TO DO", "IN PROGRESS", "TO BE TESTED", "DONE"}
	if len(gotNames) != len(wantNames) {
		t.Fatalf("status names = %v, want %v", gotNames, wantNames)
	}
	for i := range wantNames {
		if gotNames[i] != wantNames[i] {
			t.Fatalf("status order = %v, want %v", gotNames, wantNames)
		}
	}

	for _, s := range wf.Statuses {
		if s.Name == "TO BE TESTED" && s.Category != CategoryInProgress {
			t.Errorf("TO BE TESTED category = %q, want %q", s.Category, CategoryInProgress)
		}
	}

	edges := collectTransitionEdges(t, svc, wf)
	want := []transitionEdge{
		{"Start Progress", "TO DO", "IN PROGRESS"},
		{"Stop Progress", "IN PROGRESS", "TO DO"},
		{"Ready to Test", "IN PROGRESS", "TO BE TESTED"},
		{"Back to In Progress", "TO BE TESTED", "IN PROGRESS"},
		{"Done", "TO BE TESTED", "DONE"},
	}
	for _, e := range want {
		if !edges[e] {
			t.Errorf("missing transition %+v (have %v)", e, edges)
		}
	}
}

func TestCreateDefaultWorkflow_WithoutReview_IsThreeState(t *testing.T) {
	db := setupWorkflowTestDB(t)
	svc := NewService(db)

	wf, err := svc.CreateDefaultWorkflow("proj-noreview", false)
	if err != nil {
		t.Fatalf("CreateDefaultWorkflow() error = %v", err)
	}

	gotNames := make([]string, 0, len(wf.Statuses))
	for _, s := range wf.Statuses {
		gotNames = append(gotNames, s.Name)
	}
	wantNames := []string{"TO DO", "IN PROGRESS", "DONE"}
	if len(gotNames) != len(wantNames) {
		t.Fatalf("status names = %v, want %v", gotNames, wantNames)
	}
	for i := range wantNames {
		if gotNames[i] != wantNames[i] {
			t.Fatalf("status order = %v, want %v", gotNames, wantNames)
		}
	}

	edges := collectTransitionEdges(t, svc, wf)
	want := []transitionEdge{
		{"Start Progress", "TO DO", "IN PROGRESS"},
		{"Stop Progress", "IN PROGRESS", "TO DO"},
		{"Done", "IN PROGRESS", "DONE"},
	}
	if len(edges) != len(want) {
		t.Fatalf("transition count = %d (%v), want %d", len(edges), edges, len(want))
	}
	for _, e := range want {
		if !edges[e] {
			t.Errorf("missing transition %+v (have %v)", e, edges)
		}
	}
}

// TestCreateDefaultWorkflowPropagatesStatusError verifies that a failure of an
// inner AddStatus is returned instead of being silently discarded. We induce it
// by migrating only the Workflow table (not WorkflowStatus): CreateWorkflow
// succeeds, then the first AddStatus Create fails. With the old `x, _ :=` code
// the discarded nil status would be dereferenced (panic); the fixed code must
// return the error.
func TestCreateDefaultWorkflowPropagatesStatusError(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file::memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}
	if err := db.AutoMigrate(&Workflow{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	svc := NewService(db)

	wf, err := svc.CreateDefaultWorkflow("proj-err", false)
	if err == nil {
		t.Fatalf("expected error when WorkflowStatus table is missing, got nil (wf=%v)", wf)
	}
	if wf != nil {
		t.Errorf("expected nil workflow on error, got %v", wf)
	}
}

func TestAddAndRemoveStatus(t *testing.T) {
	db := setupWorkflowTestDB(t)
	svc := NewService(db)

	wf, _ := svc.CreateWorkflow("proj-3", "WF")
	status, err := svc.AddStatus(wf.ID, "Review", CategoryInProgress, "#FF0000")
	if err != nil {
		t.Fatalf("AddStatus() error = %v", err)
	}
	if status.Name != "Review" {
		t.Errorf("Name = %s", status.Name)
	}
	if status.Color != "#FF0000" {
		t.Errorf("Color = %s", status.Color)
	}

	err = svc.RemoveStatus(status.WorkflowID, status.ID)
	if err != nil {
		t.Fatalf("RemoveStatus() error = %v", err)
	}
}

func TestAddAndRemoveTransition(t *testing.T) {
	db := setupWorkflowTestDB(t)
	svc := NewService(db)

	wf, _ := svc.CreateWorkflow("proj-4", "WF")
	s1, _ := svc.AddStatus(wf.ID, "Todo", CategoryTodo, "#AAA")
	s2, _ := svc.AddStatus(wf.ID, "Done", CategoryDone, "#BBB")

	tr, err := svc.AddTransition(wf.ID, s1.ID, s2.ID, "", false, false)
	if err != nil {
		t.Fatalf("AddTransition() error = %v", err)
	}
	if tr.FromStatusID != s1.ID || tr.ToStatusID != s2.ID {
		t.Error("transition status IDs mismatch")
	}

	err = svc.RemoveTransition(tr.WorkflowID, tr.ID)
	if err != nil {
		t.Fatalf("RemoveTransition() error = %v", err)
	}
}

func TestValidateTransitionRejectsInvalid(t *testing.T) {
	db := setupWorkflowTestDB(t)
	svc := NewService(db)

	wf, _ := svc.CreateWorkflow("proj-5", "WF")
	s1, _ := svc.AddStatus(wf.ID, "Todo", CategoryTodo, "#AAA")
	s2, _ := svc.AddStatus(wf.ID, "Done", CategoryDone, "#BBB")

	err := svc.ValidateTransition(wf.ID, s1.ID, s2.ID)
	if err == nil {
		t.Error("expected error for non-existent transition")
	}
}

func TestValidateTransitionAllowsExisting(t *testing.T) {
	db := setupWorkflowTestDB(t)
	svc := NewService(db)

	wf, _ := svc.CreateWorkflow("proj-6", "WF")
	s1, _ := svc.AddStatus(wf.ID, "Todo", CategoryTodo, "#AAA")
	s2, _ := svc.AddStatus(wf.ID, "Done", CategoryDone, "#BBB")
	svc.AddTransition(wf.ID, s1.ID, s2.ID, "", false, false)

	err := svc.ValidateTransition(wf.ID, s1.ID, s2.ID)
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}

func TestGetWorkflow(t *testing.T) {
	db := setupWorkflowTestDB(t)
	svc := NewService(db)

	wf, _ := svc.CreateWorkflow("proj-7", "My WF")
	svc.AddStatus(wf.ID, "S1", CategoryTodo, "#111")
	svc.AddStatus(wf.ID, "S2", CategoryDone, "#222")

	result, err := svc.GetWorkflow("proj-7")
	if err != nil {
		t.Fatalf("GetWorkflow() error = %v", err)
	}
	if len(result.Statuses) != 2 {
		t.Errorf("expected 2 statuses, got %d", len(result.Statuses))
	}
}

func TestGetWorkflowReturnsStatusesOrderedByPosition(t *testing.T) {
	db := setupWorkflowTestDB(t)
	svc := NewService(db)

	wf, _ := svc.CreateWorkflow("proj-9", "WF")
	s1, _ := svc.AddStatus(wf.ID, "Todo", CategoryTodo, "#111")
	s2, _ := svc.AddStatus(wf.ID, "InProgress", CategoryInProgress, "#222")
	s3, _ := svc.AddStatus(wf.ID, "Done", CategoryDone, "#333")

	// Reorder to: Done, Todo, InProgress.
	if err := svc.ReorderStatuses(wf.ID, []string{s3.ID, s1.ID, s2.ID}); err != nil {
		t.Fatalf("ReorderStatuses() error = %v", err)
	}

	result, err := svc.GetWorkflow("proj-9")
	if err != nil {
		t.Fatalf("GetWorkflow() error = %v", err)
	}
	if len(result.Statuses) != 3 {
		t.Fatalf("expected 3 statuses, got %d", len(result.Statuses))
	}
	got := []string{result.Statuses[0].Name, result.Statuses[1].Name, result.Statuses[2].Name}
	want := []string{"Done", "Todo", "InProgress"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("Statuses order = %v, want %v", got, want)
		}
	}
}

func TestUpdateStatus(t *testing.T) {
	db := setupWorkflowTestDB(t)
	svc := NewService(db)

	wf, _ := svc.CreateWorkflow("proj-8", "WF")
	s, _ := svc.AddStatus(wf.ID, "Old", CategoryTodo, "#111")

	updated, err := svc.UpdateStatus(s.WorkflowID, s.ID, "New", CategoryInProgress, "#222")
	if err != nil {
		t.Fatalf("UpdateStatus() error = %v", err)
	}
	if updated.Name != "New" || updated.Category != CategoryInProgress || updated.Color != "#222" {
		t.Error("status not updated correctly")
	}
}
