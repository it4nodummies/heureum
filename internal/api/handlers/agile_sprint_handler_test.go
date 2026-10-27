package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/it4nodummies/heureum/internal/api/authz"
	"github.com/it4nodummies/heureum/internal/domain/board"
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
