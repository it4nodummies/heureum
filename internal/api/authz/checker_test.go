package authz

import (
	"errors"
	"testing"

	"github.com/google/uuid"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/it4nodummies/heureum/internal/domain/permission"
	"github.com/it4nodummies/heureum/internal/domain/project"
	"github.com/it4nodummies/heureum/internal/domain/user"
)

func newDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if err := db.AutoMigrate(&user.User{}, &project.Project{}, &project.ProjectMember{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

func createUser(t *testing.T, db *gorm.DB, isAdmin bool) string {
	t.Helper()
	id := uuid.New().String()
	u := &user.User{
		ID:          id,
		Email:       id + "@example.com",
		Username:    id,
		DisplayName: id,
		IsAdmin:     isAdmin,
		IsActive:    true,
	}
	if err := db.Create(u).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}
	return id
}

func setup(t *testing.T) (chk *Checker, globalAdminID, aliceID, bobID, p1ID string) {
	t.Helper()
	db := newDB(t)
	userSvc := user.NewService(db)
	projSvc := project.NewService(db, nil)

	globalAdminID = createUser(t, db, true)
	aliceID = createUser(t, db, false)
	bobID = createUser(t, db, false)

	p1, err := projSvc.Create("Project One", "P1", "", project.TypeScrum)
	if err != nil {
		t.Fatalf("create project: %v", err)
	}
	p1ID = p1.ID

	if err := projSvc.AddMember(p1ID, aliceID, project.RoleMember); err != nil {
		t.Fatalf("add member: %v", err)
	}

	chk = New(userSvc, projSvc, nil, nil, nil, nil, nil, nil)
	return
}

func TestRequireProject(t *testing.T) {
	chk, globalAdminID, aliceID, bobID, p1ID := setup(t)

	cases := []struct {
		name    string
		uid     string
		permKey string
		want    bool
	}{
		{"global admin: delete issues", globalAdminID, permission.DeleteIssues, true},
		{"member: create issues", aliceID, permission.CreateIssues, true},
		{"member: delete issues", aliceID, permission.DeleteIssues, false},
		{"member: administer projects", aliceID, permission.AdministerProjects, false},
		{"non-member: browse projects", bobID, permission.BrowseProjects, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := chk.RequireProject(c.uid, p1ID, c.permKey)
			if (err == nil) != c.want {
				t.Errorf("RequireProject(%s,%s)=%v, want allow=%v", c.uid, c.permKey, err, c.want)
			}
		})
	}
}

func TestRequireGlobalAdmin(t *testing.T) {
	chk, globalAdminID, aliceID, _, _ := setup(t)

	if err := chk.RequireGlobalAdmin(globalAdminID); err != nil {
		t.Errorf("admin globale deve passare: %v", err)
	}
	if err := chk.RequireGlobalAdmin(aliceID); !errors.Is(err, ErrForbidden) {
		t.Errorf("non-admin deve essere ErrForbidden, got %v", err)
	}
}
