package authz

import (
	"errors"
	"testing"

	"github.com/google/uuid"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/it4nodummies/heureum/internal/domain/group"
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

// TestRequireProjectHonorsTeamRole proves RequireProject resolves the caller's
// effective role (individual ∪ team), not just the individual project_members
// role: a user reaching a project ONLY via a team inherits that team's role.
func TestRequireProjectHonorsTeamRole(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if err := db.AutoMigrate(&user.User{}, &project.Project{}, &project.ProjectMember{},
		&project.ProjectTeam{}, &group.Group{}, &group.GroupMember{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	userSvc := user.NewService(db)
	projSvc := project.NewService(db, nil)

	memberUID := createUser(t, db, false)
	viewerUID := createUser(t, db, false)

	p, err := projSvc.Create("Team Project", "TP", "", project.TypeScrum)
	if err != nil {
		t.Fatalf("create project: %v", err)
	}

	// memberUID reaches p only via the "devs" team (member role);
	// viewerUID only via the "readers" team (viewer role). Neither is in
	// project_members.
	devs := &group.Group{ID: uuid.New().String(), Name: "devs"}
	readers := &group.Group{ID: uuid.New().String(), Name: "readers"}
	if err := db.Create(devs).Error; err != nil {
		t.Fatalf("create group: %v", err)
	}
	if err := db.Create(readers).Error; err != nil {
		t.Fatalf("create group: %v", err)
	}
	if err := db.Create(&group.GroupMember{GroupID: devs.ID, UserID: memberUID}).Error; err != nil {
		t.Fatalf("group member: %v", err)
	}
	if err := db.Create(&group.GroupMember{GroupID: readers.ID, UserID: viewerUID}).Error; err != nil {
		t.Fatalf("group member: %v", err)
	}
	if err := projSvc.AddTeam(p.ID, devs.ID, project.RoleMember); err != nil {
		t.Fatalf("add team: %v", err)
	}
	if err := projSvc.AddTeam(p.ID, readers.ID, project.RoleViewer); err != nil {
		t.Fatalf("add team: %v", err)
	}

	chk := New(userSvc, projSvc, nil, nil, nil, nil, nil, nil)

	if err := chk.RequireProject(memberUID, p.ID, permission.EditIssues); err != nil {
		t.Errorf("team member should be allowed EDIT_ISSUES: %v", err)
	}
	if err := chk.RequireProject(memberUID, p.ID, permission.DeleteIssues); err == nil {
		t.Error("team member must NOT be allowed DELETE_ISSUES")
	}
	if err := chk.RequireProject(viewerUID, p.ID, permission.EditIssues); err == nil {
		t.Error("team viewer must NOT be allowed EDIT_ISSUES")
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
