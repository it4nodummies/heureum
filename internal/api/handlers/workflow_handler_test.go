package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/open-jira/open-jira/internal/domain/issue"
	"github.com/open-jira/open-jira/internal/domain/workflow"
)

func setupWorkflowHandlerTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file::memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}
	db.AutoMigrate(&workflow.Workflow{}, &workflow.WorkflowStatus{}, &workflow.WorkflowTransition{}, &issue.Issue{}, &issue.IssueType{})
	return db
}

func TestGetWorkflowHandler(t *testing.T) {
	db := setupWorkflowHandlerTestDB(t)
	wfSvc := workflow.NewService(db)
	wfSvc.CreateDefaultWorkflow("proj-1")
	h := NewWorkflowHandler(wfSvc, nil)

	req := httptest.NewRequest("GET", "/api/v1/projects/TEST/workflow", nil)
	req.SetPathValue("key", "proj-1")
	w := httptest.NewRecorder()
	h.GetWorkflow(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var result workflow.Workflow
	json.NewDecoder(w.Body).Decode(&result)
	if len(result.Statuses) != 3 {
		t.Errorf("expected 3 statuses, got %d", len(result.Statuses))
	}
}

func TestAddStatusHandler(t *testing.T) {
	db := setupWorkflowHandlerTestDB(t)
	wfSvc := workflow.NewService(db)
	wf, _ := wfSvc.CreateWorkflow("proj-2", "WF")
	h := NewWorkflowHandler(wfSvc, nil)

	body := map[string]string{"name": "Review", "category": "inprogress", "color": "#FF0000"}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/api/v1/projects/PROJ/workflow/statuses", bytes.NewReader(b))
	req.SetPathValue("key", wf.ProjectID)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.AddStatus(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
}

func TestUpdateStatusHandler(t *testing.T) {
	db := setupWorkflowHandlerTestDB(t)
	wfSvc := workflow.NewService(db)
	wf, _ := wfSvc.CreateWorkflow("proj-3", "WF")
	s, _ := wfSvc.AddStatus(wf.ID, "Old", workflow.CategoryTodo, "#111")
	h := NewWorkflowHandler(wfSvc, nil)

	body := map[string]string{"name": "New", "category": "inprogress", "color": "#222"}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest("PATCH", "/api/v1/projects/PROJ/workflow/statuses/"+s.ID, bytes.NewReader(b))
	req.SetPathValue("key", wf.ProjectID)
	req.SetPathValue("id", s.ID)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.UpdateStatus(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var status workflow.WorkflowStatus
	json.NewDecoder(w.Body).Decode(&status)
	if status.Name != "New" {
		t.Errorf("Name = %s", status.Name)
	}
}

func TestDeleteStatusHandler(t *testing.T) {
	db := setupWorkflowHandlerTestDB(t)
	wfSvc := workflow.NewService(db)
	wf, _ := wfSvc.CreateWorkflow("proj-4", "WF")
	s, _ := wfSvc.AddStatus(wf.ID, "Extra", workflow.CategoryTodo, "#111")
	h := NewWorkflowHandler(wfSvc, nil)

	req := httptest.NewRequest("DELETE", "/api/v1/projects/PROJ/workflow/statuses/"+s.ID, nil)
	req.SetPathValue("key", wf.ProjectID)
	req.SetPathValue("id", s.ID)
	w := httptest.NewRecorder()
	h.DeleteStatus(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("expected 204, got %d: %s", w.Code, w.Body.String())
	}
}

func TestAddTransitionHandler(t *testing.T) {
	db := setupWorkflowHandlerTestDB(t)
	wfSvc := workflow.NewService(db)
	wf, _ := wfSvc.CreateWorkflow("proj-5", "WF")
	s1, _ := wfSvc.AddStatus(wf.ID, "Todo", workflow.CategoryTodo, "#AAA")
	s2, _ := wfSvc.AddStatus(wf.ID, "Done", workflow.CategoryDone, "#BBB")
	h := NewWorkflowHandler(wfSvc, nil)

	body := map[string]string{"from_status_id": s1.ID, "to_status_id": s2.ID}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/api/v1/projects/PROJ/workflow/transitions", bytes.NewReader(b))
	req.SetPathValue("key", wf.ProjectID)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.AddTransition(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
}

func TestTransitionIssueHandler(t *testing.T) {
	db := setupWorkflowHandlerTestDB(t)
	wfSvc := workflow.NewService(db)
	wfSvc.CreateDefaultWorkflow("proj-6")

	issueSvc := issue.NewService(db)
	wfForIssue, _ := wfSvc.GetWorkflow("proj-6")
	iss, _ := issueSvc.Create("PROJ", "proj-6", "Test", "desc", issue.PriorityMedium, nil, nil)
	iss, _ = issueSvc.Update(iss.Key, nil, nil, nil, nil, &wfForIssue.Statuses[0].ID, nil)

	h := NewWorkflowHandler(wfSvc, issueSvc)

	progStatus := wfForIssue.Statuses[1]

	body := map[string]string{"to_status_id": progStatus.ID}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/api/v1/issues/"+iss.Key+"/transition", bytes.NewReader(b))
	req.SetPathValue("issueKey", iss.Key)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.TransitionIssue(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestTransitionIssueHandlerInvalid(t *testing.T) {
	db := setupWorkflowHandlerTestDB(t)
	wfSvc := workflow.NewService(db)
	wfSvc.CreateDefaultWorkflow("proj-7")
	wfForIssue, _ := wfSvc.GetWorkflow("proj-7")

	issueSvc := issue.NewService(db)
	iss, _ := issueSvc.Create("PROJ", "proj-7", "Test", "desc", issue.PriorityMedium, nil, nil)

	h := NewWorkflowHandler(wfSvc, issueSvc)

	doneStatus := wfForIssue.Statuses[2]

	body := map[string]string{"to_status_id": doneStatus.ID}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/api/v1/issues/"+iss.Key+"/transition", bytes.NewReader(b))
	req.SetPathValue("issueKey", iss.Key)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.TransitionIssue(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}
