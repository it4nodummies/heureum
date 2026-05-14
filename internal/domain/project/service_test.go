package project

import (
	"testing"

	"github.com/google/uuid"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/open-jira/open-jira/internal/domain/user"
)

func setupTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file::memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}
	db.AutoMigrate(&user.User{}, &Project{}, &ProjectMember{})
	return db
}

func TestCreateProject(t *testing.T) {
	db := setupTestDB(t)
	svc := NewService(db, &user.User{ID: uuid.New().String()})
	p, err := svc.Create("Test Project", "TEST", "A test project", TypeScrum)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if p.Name != "Test Project" {
		t.Errorf("Name = %s", p.Name)
	}
	if p.Key != "TEST" {
		t.Errorf("Key = %s", p.Key)
	}
}

func TestCreateProjectDuplicateKey(t *testing.T) {
	db := setupTestDB(t)
	lead := &user.User{ID: uuid.New().String()}
	db.Create(lead)
	svc := NewService(db, lead)
	svc.Create("A", "DUP", "desc", TypeScrum)
	_, err := svc.Create("B", "DUP", "desc", TypeScrum)
	if err == nil {
		t.Error("expected duplicate key error")
	}
}

func TestListProjectsSkipsArchived(t *testing.T) {
	db := setupTestDB(t)
	svc := NewService(db, &user.User{ID: uuid.New().String()})
	svc.Create("P1", "P1", "desc", TypeScrum)
	p2, _ := svc.Create("P2", "P2", "desc", TypeKanban)
	svc.Archive("P2")
	projects, _ := svc.List(false)
	if len(projects) != 1 {
		t.Errorf("expected 1 active project, got %d", len(projects))
	}
	_ = p2
}
