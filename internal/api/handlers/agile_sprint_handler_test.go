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
