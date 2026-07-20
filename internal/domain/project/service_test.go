package project

import (
	"errors"
	"testing"

	"github.com/google/uuid"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/it4nodummies/heureum/internal/domain/group"
	"github.com/it4nodummies/heureum/internal/domain/user"
)

func setupTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file::memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}
	// ProjectTeam + group tables are required because MembershipSubquery unions
	// team-inherited membership (project_teams ⋈ group_members).
	db.AutoMigrate(&user.User{}, &Project{}, &ProjectMember{}, &ProjectTeam{},
		&group.Group{}, &group.GroupMember{})
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

func TestCreateProjectWithInput(t *testing.T) {
	db := setupTestDB(t)
	svc := NewService(db, nil)
	p, err := svc.CreateProject(CreateInput{Key: "NEW", Name: "New Project", Description: "d", Type: TypeKanban, AssigneeType: "PROJECT_LEAD"})
	if err != nil {
		t.Fatal(err)
	}
	if p.Key != "NEW" || p.Type != TypeKanban || p.AssigneeType != "PROJECT_LEAD" {
		t.Errorf("unexpected: %+v", p)
	}
}

func TestCreateProjectInvalidKey(t *testing.T) {
	db := setupTestDB(t)
	svc := NewService(db, nil)
	// All-numeric and otherwise malformed keys must be rejected so a key can
	// never shadow a numeric seq_id lookup in GET.
	for _, k := range []string{"99999", "1", "A", "toolongkey1", "A-B", "9AB"} {
		if _, err := svc.CreateProject(CreateInput{Key: k, Name: "X"}); !errors.Is(err, ErrInvalidKey) {
			t.Errorf("key %q: err = %v, want ErrInvalidKey", k, err)
		}
	}
	// Legacy Create path shares the same rule.
	if _, err := svc.Create("X", "12345", "d", TypeScrum); !errors.Is(err, ErrInvalidKey) {
		t.Errorf("Create numeric key: err = %v, want ErrInvalidKey", err)
	}
	// A valid key still succeeds.
	if _, err := svc.CreateProject(CreateInput{Key: "OK1", Name: "X"}); err != nil {
		t.Errorf("valid key OK1: unexpected err = %v", err)
	}
}

func TestArchiveThenRestore(t *testing.T) {
	db := setupTestDB(t)
	svc := NewService(db, nil)
	if _, err := svc.CreateProject(CreateInput{Key: "ARC", Name: "Arc", Type: TypeScrum}); err != nil {
		t.Fatal(err)
	}
	if err := svc.Archive("ARC"); err != nil {
		t.Fatal(err)
	}
	got, _ := svc.GetByKey("ARC")
	if !got.IsArchived {
		t.Fatal("expected archived")
	}
	if err := svc.Restore("ARC"); err != nil {
		t.Fatal(err)
	}
	got, _ = svc.GetByKey("ARC")
	if got.IsArchived {
		t.Error("expected restored")
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

func TestGetRole_ReturnsEmptyForNonMember(t *testing.T) {
	db := setupTestDB(t)
	svc := NewService(db, nil)
	role, err := svc.GetRole("proj-x", "user-x")
	if err != nil {
		t.Fatalf("GetRole err: %v", err)
	}
	if role != "" {
		t.Errorf("expected empty role for non-member, got %q", role)
	}
}

func TestAddMember_IsIdempotentAndUpdatesRole(t *testing.T) {
	db := setupTestDB(t)
	svc := NewService(db, nil)
	if err := svc.AddMember("p1", "u1", RoleMember); err != nil {
		t.Fatal(err)
	}
	if err := svc.AddMember("p1", "u1", RoleAdmin); err != nil {
		t.Fatalf("re-add must be idempotent: %v", err)
	}
	role, _ := svc.GetRole("p1", "u1")
	if role != RoleAdmin {
		t.Errorf("expected admin after upsert, got %q", role)
	}
	var cnt int64
	svc.DB().Model(&ProjectMember{}).Where("project_id = ? AND user_id = ?", "p1", "u1").Count(&cnt)
	if cnt != 1 {
		t.Errorf("expected 1 member row, got %d", cnt)
	}
}

func TestMembershipSubquery_FiltersToMemberProjects(t *testing.T) {
	db := setupTestDB(t)
	svc := NewService(db, nil)

	p1, err := svc.CreateProject(CreateInput{Key: "MSQ1", Name: "One", Type: TypeScrum})
	if err != nil {
		t.Fatal(err)
	}
	p2, err := svc.CreateProject(CreateInput{Key: "MSQ2", Name: "Two", Type: TypeScrum})
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.AddMember(p1.ID, "user-1", RoleMember); err != nil {
		t.Fatal(err)
	}

	sub := svc.MembershipSubquery("user-1")

	var got []Project
	if err := db.Where("id IN (?)", sub).Find(&got).Error; err != nil {
		t.Fatalf("query with subquery: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 project for user-1, got %d", len(got))
	}
	if got[0].ID != p1.ID {
		t.Errorf("expected project %s, got %s", p1.ID, got[0].ID)
	}
	_ = p2
}

func TestListWithFilters_ScopesByMembership(t *testing.T) {
	db := setupTestDB(t)
	svc := NewService(db, nil)

	p1, err := svc.CreateProject(CreateInput{Key: "LWF1", Name: "One", Type: TypeScrum})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.CreateProject(CreateInput{Key: "LWF2", Name: "Two", Type: TypeScrum}); err != nil {
		t.Fatal(err)
	}
	if err := svc.AddMember(p1.ID, "user-1", RoleMember); err != nil {
		t.Fatal(err)
	}

	// Scoped to user-1: only the project they're a member of.
	rows, total, err := svc.ListWithFilters(ListFilter{MemberUserID: "user-1"}, "user-1")
	if err != nil {
		t.Fatalf("ListWithFilters: %v", err)
	}
	if total != 1 || len(rows) != 1 {
		t.Fatalf("expected 1 scoped project, got total=%d rows=%d", total, len(rows))
	}
	if rows[0].ID != p1.ID {
		t.Errorf("expected project %s, got %s", p1.ID, rows[0].ID)
	}

	// Unscoped (empty MemberUserID, e.g. global admin): both projects.
	rows, total, err = svc.ListWithFilters(ListFilter{}, "user-1")
	if err != nil {
		t.Fatalf("ListWithFilters: %v", err)
	}
	if total != 2 || len(rows) != 2 {
		t.Fatalf("expected 2 unscoped projects, got total=%d rows=%d", total, len(rows))
	}
}

func TestCreateProject_AddsCreatorAsAdmin(t *testing.T) {
	db := setupTestDB(t)
	svc := NewService(db, nil)
	p, err := svc.CreateProject(CreateInput{Name: "P", Key: "CREATP", Type: TypeScrum, CreatorID: "creator-1"})
	if err != nil {
		t.Fatal(err)
	}
	role, _ := svc.GetRole(p.ID, "creator-1")
	if role != RoleAdmin {
		t.Errorf("creator must be admin, got %q", role)
	}
}
