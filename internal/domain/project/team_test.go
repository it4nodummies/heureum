package project

import (
	"testing"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/it4nodummies/heureum/internal/domain/group"
	"github.com/it4nodummies/heureum/internal/domain/user"
)

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
	db := setupTestDB(t)
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
	db := setupTestDB(t)
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

func containsID(ids []string, id string) bool {
	for _, x := range ids {
		if x == id {
			return true
		}
	}
	return false
}

func TestMembershipIncludesTeamProjects(t *testing.T) {
	db := setupTestDB(t)
	s := NewService(db, nil)
	p := seedProject(t, db, "VIA")
	g := seedGroup(t, db, "teamg")
	u := &user.User{ID: uuid.NewString()}
	if err := db.Create(u).Error; err != nil {
		t.Fatalf("seed user: %v", err)
	}
	if err := db.Create(&group.GroupMember{GroupID: g.ID, UserID: u.ID}).Error; err != nil {
		t.Fatalf("seed group member: %v", err)
	}
	// u is NOT an individual member of p, but is in g which is a team of p.
	if err := s.AddTeam(p.ID, g.ID, RoleViewer); err != nil {
		t.Fatal(err)
	}

	ids := s.MemberProjectIDs(u.ID)
	if !containsID(ids, p.ID) {
		t.Fatalf("want project reachable via team in membership, got %v", ids)
	}
}

// TestListWithFilters_ScopesByTeamMembership is the end-to-end analogue of
// TestListWithFilters_ScopesByMembership: it proves that a project reachable
// ONLY via a team (never via project_members) surfaces in ListWithFilters when
// scoped by MemberUserID, exercising the real `id IN (?)` subquery embedding.
func TestListWithFilters_ScopesByTeamMembership(t *testing.T) {
	db := setupTestDB(t)
	svc := NewService(db, nil)

	// p1 reachable only via team; p2 not reachable at all.
	p1, err := svc.CreateProject(CreateInput{Key: "TWF1", Name: "One", Type: TypeScrum})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.CreateProject(CreateInput{Key: "TWF2", Name: "Two", Type: TypeScrum}); err != nil {
		t.Fatal(err)
	}

	g := seedGroup(t, db, "twf-team")
	u := &user.User{ID: uuid.NewString()}
	if err := db.Create(u).Error; err != nil {
		t.Fatalf("seed user: %v", err)
	}
	if err := db.Create(&group.GroupMember{GroupID: g.ID, UserID: u.ID}).Error; err != nil {
		t.Fatalf("seed group member: %v", err)
	}
	// u reaches p1 ONLY through the team association, not project_members.
	if err := svc.AddTeam(p1.ID, g.ID, RoleMember); err != nil {
		t.Fatal(err)
	}

	rows, total, err := svc.ListWithFilters(ListFilter{MemberUserID: u.ID}, u.ID)
	if err != nil {
		t.Fatalf("ListWithFilters: %v", err)
	}
	if total != 1 || len(rows) != 1 {
		t.Fatalf("expected 1 team-scoped project, got total=%d rows=%d", total, len(rows))
	}
	if rows[0].ID != p1.ID {
		t.Errorf("expected project %s reachable via team, got %s", p1.ID, rows[0].ID)
	}
}
