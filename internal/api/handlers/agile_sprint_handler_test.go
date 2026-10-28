package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/google/uuid"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/it4nodummies/heureum/internal/api/authz"
	"github.com/it4nodummies/heureum/internal/domain/board"
	"github.com/it4nodummies/heureum/internal/domain/issue"
	"github.com/it4nodummies/heureum/internal/domain/project"
	"github.com/it4nodummies/heureum/internal/domain/sprint"
	"github.com/it4nodummies/heureum/internal/domain/user"
	"github.com/it4nodummies/heureum/internal/storetest"
)

// setupAgileSprintTest costruisce un DB in-memory con user admin, servizi e
// l'AgileSprintHandler. Restituisce handler, DB e l'uid admin (globale, così
// RequireProject passa senza dover configurare ruoli di progetto).
func setupAgileSprintTest(t *testing.T) (*AgileSprintHandler, *gorm.DB, string) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file::memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(
		&user.User{},
		&project.Project{}, &project.ProjectMember{}, &project.Invite{},
		&board.Board{},
		&sprint.Sprint{},
	); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	adminUID := uuid.New().String()
	db.Create(&user.User{ID: adminUID, Email: "admin@example.com", Username: adminUID, DisplayName: "Admin", IsActive: true, IsAdmin: true})

	userSvc := user.NewService(db)
	projSvc := project.NewService(db, &user.User{ID: adminUID})
	boardSvc := board.NewService(db)
	sprintSvc := sprint.NewService(db)
	chk := authz.New(userSvc, projSvc, nil, boardSvc, sprintSvc, nil, nil, nil)

	h := NewAgileSprintHandler(sprintSvc, boardSvc, nil, nil, chk, "http://localhost:8080")
	return h, db, adminUID
}

// seedBoard crea un progetto + una board del tipo richiesto e ne restituisce il seq_id.
func seedBoard(t *testing.T, db *gorm.DB, boardSvc *board.Service, boardType string) int64 {
	t.Helper()
	p := &project.Project{ID: uuid.New().String(), Key: "DEMO", Name: "Demo", SeqID: 10000}
	if err := db.Create(p).Error; err != nil {
		t.Fatalf("create project: %v", err)
	}
	b, err := boardSvc.Create(p.ID, "Board", boardType, nil)
	if err != nil {
		t.Fatalf("create board: %v", err)
	}
	return b.SeqID
}

// TestAgileSprintCreate_RejectedOnKanbanBoard: una board kanban deve rifiutare la
// creazione di uno sprint con 400 e nessuna riga sprint persistita.
func TestAgileSprintCreate_RejectedOnKanbanBoard(t *testing.T) {
	h, db, adminUID := setupAgileSprintTest(t)
	boardSeq := seedBoard(t, db, h.boardSvc, "kanban")

	body, _ := json.Marshal(map[string]any{"name": "Sprint 1", "originBoardId": boardSeq})
	req := withUID(httptest.NewRequest(http.MethodPost, "/rest/agile/1.0/sprint", bytes.NewReader(body)), adminUID)
	w := httptest.NewRecorder()
	h.Create(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("create on kanban board status = %d, want 400: %s", w.Code, w.Body.String())
	}
	var count int64
	db.Model(&sprint.Sprint{}).Count(&count)
	if count != 0 {
		t.Fatalf("expected no sprint persisted on kanban board, got %d", count)
	}
}

// TestAgileSprintCreate_AllowedOnScrumBoard: una board scrum continua a creare lo
// sprint (201) e a persistere la riga.
func TestAgileSprintCreate_AllowedOnScrumBoard(t *testing.T) {
	h, db, adminUID := setupAgileSprintTest(t)
	boardSeq := seedBoard(t, db, h.boardSvc, "scrum")

	body, _ := json.Marshal(map[string]any{"name": "Sprint 1", "originBoardId": boardSeq})
	req := withUID(httptest.NewRequest(http.MethodPost, "/rest/agile/1.0/sprint", bytes.NewReader(body)), adminUID)
	w := httptest.NewRecorder()
	h.Create(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("create on scrum board status = %d, want 201: %s", w.Code, w.Body.String())
	}
	var count int64
	db.Model(&sprint.Sprint{}).Count(&count)
	if count != 1 {
		t.Fatalf("expected one sprint persisted on scrum board, got %d", count)
	}
}

// reloadIssue rilegge una issue dal DB del fixture A/B.
func (f *nestedFixture) reloadIssue(t *testing.T, id string) issue.Issue {
	t.Helper()
	var got issue.Issue
	if err := f.db.First(&got, "id = ?", id).Error; err != nil {
		t.Fatalf("reload issue: %v", err)
	}
	return got
}

// TestMoveToSprint_RejectsIssueFromOtherProject: the path authorizes
// ManageSprints on the SPRINT's project (A); the body names issues. An issue of
// B (where the caller is only a viewer) must be rejected with 400 and — because
// the whole batch is validated before anything is written — issue A, listed
// first, must NOT have been moved either. The positive case (only A's issue)
// still moves it.
func TestMoveToSprint_RejectsIssueFromOtherProject(t *testing.T) {
	f := setupNestedFixture(t)
	spA, err := f.sprintSvc.Create(f.projA.ID, "Sprint A", "")
	if err != nil {
		t.Fatalf("create sprint: %v", err)
	}
	path := "/rest/agile/1.0/sprint/" + strconv.FormatInt(spA.SeqID, 10) + "/issue"

	// Mixed batch: A's issue first (would be moved by a naive loop), then B's.
	w := f.do(t, f.adminA, http.MethodPost, path, `{"issues":["`+f.issueA.Key+`","`+f.issueB.Key+`"]}`)
	f.expect(t, "move mixed-project batch", w, http.StatusBadRequest)
	if got := f.reloadIssue(t, f.issueA.ID); got.SprintID != nil {
		t.Errorf("partial move: issue A was moved into the sprint although the batch was rejected")
	}
	if got := f.reloadIssue(t, f.issueB.ID); got.SprintID != nil {
		t.Errorf("issue B pulled into A's sprint across project boundary")
	}

	// By seq id too (ids are guessable: 10001...).
	w = f.do(t, f.adminA, http.MethodPost, path, `{"issues":["`+strconv.FormatInt(f.issueB.SeqID, 10)+`"]}`)
	f.expect(t, "move other-project issue by id", w, http.StatusBadRequest)
	if got := f.reloadIssue(t, f.issueB.ID); got.SprintID != nil {
		t.Errorf("issue B pulled into A's sprint by seq id")
	}

	// Positive: an issue of the sprint's own project moves.
	w = f.do(t, f.adminA, http.MethodPost, path, `{"issues":["`+f.issueA.Key+`"]}`)
	f.expect(t, "move own-project issue", w, http.StatusNoContent)
	if got := f.reloadIssue(t, f.issueA.ID); got.SprintID == nil || *got.SprintID != spA.ID {
		t.Errorf("issue A not moved into its own project's sprint: %+v", got.SprintID)
	}
}

// TestCompleteSprint_RejectsMoveToOtherProjectSprint: the path authorizes
// ManageSprints on sprint A's project; the body-sourced moveToSprintId must be
// a sprint of that same project. A sprint of B (where the caller is only a
// viewer) is a 400 and nothing is written: sprint A stays active and its open
// issue stays in it. Moving into another sprint of A still works.
func TestCompleteSprint_RejectsMoveToOtherProjectSprint(t *testing.T) {
	f := setupNestedFixture(t)
	spA, err := f.sprintSvc.Create(f.projA.ID, "Sprint A", "")
	if err != nil {
		t.Fatalf("create sprint A: %v", err)
	}
	spA2, err := f.sprintSvc.Create(f.projA.ID, "Sprint A2", "")
	if err != nil {
		t.Fatalf("create sprint A2: %v", err)
	}
	spB, err := f.sprintSvc.Create(f.projB.ID, "Sprint B", "")
	if err != nil {
		t.Fatalf("create sprint B: %v", err)
	}
	if _, err := f.sprintSvc.Start(f.projA.ID, spA.ID); err != nil {
		t.Fatalf("start sprint A: %v", err)
	}
	if err := f.sprintSvc.AddIssue(spA.ID, f.issueA.ID); err != nil {
		t.Fatalf("add issue A to sprint A: %v", err)
	}
	// Complete only moves *open* issues (status category != done); the raw
	// fixture issue has no status at all, so park it in A's first (todo) status.
	wfA, err := f.wfSvc.GetWorkflow(f.projA.ID)
	if err != nil || len(wfA.Statuses) == 0 {
		t.Fatalf("workflow A: %v", err)
	}
	if err := f.db.Model(&issue.Issue{}).Where("id = ?", f.issueA.ID).Update("status_id", wfA.Statuses[0].ID).Error; err != nil {
		t.Fatalf("set issue A status: %v", err)
	}
	path := "/rest/agile/1.0/sprint/" + strconv.FormatInt(spA.SeqID, 10) + "/complete"

	w := f.do(t, f.adminA, http.MethodPost, path, `{"moveToSprintId":`+strconv.FormatInt(spB.SeqID, 10)+`}`)
	f.expect(t, "complete with other-project target", w, http.StatusBadRequest)
	if got := f.reloadIssue(t, f.issueA.ID); got.SprintID == nil || *got.SprintID != spA.ID {
		t.Errorf("issue A moved although the request was rejected: sprint_id=%v", got.SprintID)
	}
	if got, err := f.sprintSvc.GetByID(f.projA.ID, spA.ID); err != nil || got.State != sprint.StateActive {
		t.Errorf("sprint A state = %v (err %v), want still active (no partial write)", got, err)
	}

	// Positive: a sprint of the same project.
	w = f.do(t, f.adminA, http.MethodPost, path, `{"moveToSprintId":`+strconv.FormatInt(spA2.SeqID, 10)+`}`)
	f.expect(t, "complete with own-project target", w, http.StatusOK)
	if got := f.reloadIssue(t, f.issueA.ID); got.SprintID == nil || *got.SprintID != spA2.ID {
		t.Errorf("issue A not moved into sprint A2: sprint_id=%v", got.SprintID)
	}
}

// TestCompleteSprint_DBErrorIs500: a real DB failure while completing the
// sprint (here: the issues table is gone, so moving the open issues fails)
// is a 500, not a 400 — and, since completion is one transaction, the sprint
// is still active afterwards. Runs on the real migrated schema.
func TestCompleteSprint_DBErrorIs500(t *testing.T) {
	db := storetest.New(t)
	adminUID := uuid.New().String()
	if err := db.Create(&user.User{ID: adminUID, Email: "admin@example.test", Username: adminUID, DisplayName: "Admin", IsActive: true, IsAdmin: true}).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}
	p := &project.Project{ID: uuid.New().String(), Key: "SPR", Name: "Sprints", SeqID: 10000}
	if err := db.Create(p).Error; err != nil {
		t.Fatalf("create project: %v", err)
	}
	sprintSvc := sprint.NewService(db)
	sp, err := sprintSvc.CreateFull(p.ID, "Sprint 1", "", nil, nil, nil)
	if err != nil {
		t.Fatalf("create sprint: %v", err)
	}
	if _, err := sprintSvc.Start(p.ID, sp.ID); err != nil {
		t.Fatalf("start sprint: %v", err)
	}
	projSvc := project.NewService(db, &user.User{ID: adminUID})
	chk := authz.New(user.NewService(db), projSvc, nil, board.NewService(db), sprintSvc, nil, nil, nil)
	h := NewAgileSprintHandler(sprintSvc, board.NewService(db), nil, nil, chk, "http://localhost:8080")

	if err := db.Exec("ALTER TABLE issues RENAME TO issues_gone").Error; err != nil {
		t.Fatalf("rename issues: %v", err)
	}

	seq := strconv.FormatInt(sp.SeqID, 10)
	req := withUID(httptest.NewRequest(http.MethodPost, "/rest/agile/1.0/sprint/"+seq+"/complete", bytes.NewReader([]byte(`{"moveOpenToBacklog":true}`))), adminUID)
	req.SetPathValue("sprintId", seq)
	w := httptest.NewRecorder()
	h.CompleteSprint(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500: %s", w.Code, w.Body.String())
	}
	got, err := sprintSvc.GetByID(p.ID, sp.ID)
	if err != nil {
		t.Fatalf("reload sprint: %v", err)
	}
	if got.State != sprint.StateActive {
		t.Errorf("sprint state = %q after a failed completion, want active (rolled back)", got.State)
	}
}
