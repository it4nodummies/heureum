package project

import (
	"testing"

	"github.com/google/uuid"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/it4nodummies/heureum/internal/domain/group"
	"github.com/it4nodummies/heureum/internal/domain/user"
)

// setupTeamTestDB opens an in-memory SQLite DB migrating the project, member,
// team and group models needed for team-association tests.
func setupTeamTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file::memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}
	if err := db.AutoMigrate(
		&user.User{}, &Project{}, &ProjectMember{}, &ProjectTeam{},
		&group.Group{}, &group.GroupMember{},
	); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

// seedProject inserts a project directly and returns it.
func seedProject(t *testing.T, db *gorm.DB, key string) *Project {
	t.Helper()
	p := &Project{ID: uuid.NewString(), Key: key, Name: key + " project"}
	if err := db.Create(p).Error; err != nil {
		t.Fatalf("seed project: %v", err)
	}
	return p
}

// seedGroup inserts a group directly and returns it.
func seedGroup(t *testing.T, db *gorm.DB, name string) *group.Group {
	t.Helper()
	g := &group.Group{ID: uuid.NewString(), Name: name}
	if err := db.Create(g).Error; err != nil {
		t.Fatalf("seed group: %v", err)
	}
	return g
}

func TestAddTeamIsIdempotentAndListTeams(t *testing.T) {
	db := setupTeamTestDB(t)
	s := NewService(db, nil)
	p := seedProject(t, db, "TEAM")
	g := seedGroup(t, db, "developers")

	if err := s.AddTeam(p.ID, g.ID, RoleMember); err != nil {
		t.Fatalf("AddTeam member: %v", err)
	}
	if err := s.AddTeam(p.ID, g.ID, RoleAdmin); err != nil {
		t.Fatalf("AddTeam admin (upsert): %v", err)
	}

	teams, err := s.ListTeams(p.ID)
	if err != nil {
		t.Fatalf("ListTeams: %v", err)
	}
	if len(teams) != 1 || teams[0].Role != RoleAdmin || teams[0].GroupID != g.ID {
		t.Fatalf("want 1 team admin, got %+v", teams)
	}
	if teams[0].GroupName != g.Name {
		t.Fatalf("want group name hydrated %q, got %q", g.Name, teams[0].GroupName)
	}

	if err := s.RemoveTeam(p.ID, g.ID); err != nil {
		t.Fatalf("RemoveTeam: %v", err)
	}
	teams, _ = s.ListTeams(p.ID)
	if len(teams) != 0 {
		t.Fatalf("want 0 after remove, got %d", len(teams))
	}
}

func TestEffectiveRoleMostPermissive(t *testing.T) {
	db := setupTeamTestDB(t)
	s := NewService(db, nil)
	p := seedProject(t, db, "EFF")
	g := seedGroup(t, db, "devs")
	u := &user.User{ID: uuid.NewString()}
	if err := db.Create(u).Error; err != nil {
		t.Fatalf("seed user: %v", err)
	}
	if err := db.Create(&group.GroupMember{GroupID: g.ID, UserID: u.ID}).Error; err != nil {
		t.Fatalf("seed group member: %v", err)
	}

	// solo team member -> member
	if err := s.AddTeam(p.ID, g.ID, RoleMember); err != nil {
		t.Fatal(err)
	}
	if r, ok := s.EffectiveRole(u.ID, p.ID); !ok || r != RoleMember {
		t.Fatalf("solo team: want member, got %v ok=%v", r, ok)
	}

	// individuale viewer + team member -> member (più permissivo)
	if err := s.AddMember(p.ID, u.ID, RoleViewer); err != nil {
		t.Fatal(err)
	}
	if r, ok := s.EffectiveRole(u.ID, p.ID); !ok || r != RoleMember {
		t.Fatalf("viewer+team member: want member, got %v", r)
	}

	// team admin -> admin vince
	if err := s.AddTeam(p.ID, g.ID, RoleAdmin); err != nil {
		t.Fatal(err)
	}
	if r, _ := s.EffectiveRole(u.ID, p.ID); r != RoleAdmin {
		t.Fatalf("team admin: want admin, got %v", r)
	}

	// utente senza accesso
	if _, ok := s.EffectiveRole("nobody", p.ID); ok {
		t.Fatal("no access: want ok=false")
	}
}
