package contract

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/google/uuid"

	"github.com/it4nodummies/heureum/internal/domain/group"
	"github.com/it4nodummies/heureum/internal/domain/user"
)

// TestProjectTeams_AdminAssociatesTeam_MemberForbidden exercises the Heureum
// extension endpoints /project/{key}/teams end-to-end through the real router
// (authz included): the project admin (creator) can associate a team and read
// it back, while a plain project member — who lacks ADMINISTER_PROJECTS — is
// forbidden from associating one.
func TestProjectTeams_AdminAssociatesTeam_MemberForbidden(t *testing.T) {
	srv, authSvc, db := newTestServerDB(t)
	alice := registerAndLogin(t, authSvc) // creator => project admin
	bob := registerUserAndLogin(t, authSvc, "bob@example.com", "bob")
	createProjectViaAPI(t, srv, alice, "TM", "Team Proj")

	// A group to associate (POST /group requires global admin; seed it directly,
	// as other authz tests set DB state directly).
	g := &group.Group{ID: uuid.NewString(), Name: "developers"}
	if err := db.Create(g).Error; err != nil {
		t.Fatalf("seed group: %v", err)
	}

	// Add bob as a plain project member so the 403 asserts on role, not on
	// non-membership.
	var bobUser user.User
	if err := db.First(&bobUser, "email = ?", "bob@example.com").Error; err != nil {
		t.Fatalf("lookup bob: %v", err)
	}
	resAdd := doJSON(t, srv, http.MethodPost, alice, "/rest/api/3/project/TM/members", map[string]any{
		"user_id": bobUser.ID, "role": "member",
	})
	if resAdd.StatusCode != http.StatusCreated {
		t.Fatalf("add bob as member status = %d, want 201", resAdd.StatusCode)
	}

	// Admin associates the team → 204.
	resAssoc := doJSON(t, srv, http.MethodPost, alice, "/rest/api/3/project/TM/teams", map[string]any{
		"groupId": g.ID, "role": "member",
	})
	if resAssoc.StatusCode != http.StatusNoContent {
		t.Fatalf("POST /project/TM/teams (admin) status = %d, want 204", resAssoc.StatusCode)
	}

	// Bad role → 400.
	resBad := doJSON(t, srv, http.MethodPost, alice, "/rest/api/3/project/TM/teams", map[string]any{
		"groupId": g.ID, "role": "superuser",
	})
	if resBad.StatusCode != http.StatusBadRequest {
		t.Fatalf("POST /project/TM/teams (bad role) status = %d, want 400", resBad.StatusCode)
	}

	// Unknown group → 400.
	resNoGroup := doJSON(t, srv, http.MethodPost, alice, "/rest/api/3/project/TM/teams", map[string]any{
		"groupId": uuid.NewString(), "role": "member",
	})
	if resNoGroup.StatusCode != http.StatusBadRequest {
		t.Fatalf("POST /project/TM/teams (unknown group) status = %d, want 400", resNoGroup.StatusCode)
	}

	// GET returns the associated team.
	resList := doJSON(t, srv, http.MethodGet, alice, "/rest/api/3/project/TM/teams", nil)
	if resList.StatusCode != http.StatusOK {
		t.Fatalf("GET /project/TM/teams (admin) status = %d, want 200", resList.StatusCode)
	}
	var teams []struct {
		GroupID string `json:"groupId"`
		Name    string `json:"name"`
		Role    string `json:"role"`
	}
	if err := json.NewDecoder(resList.Body).Decode(&teams); err != nil {
		t.Fatal(err)
	}
	resList.Body.Close()
	if len(teams) != 1 || teams[0].GroupID != g.ID || teams[0].Name != "developers" || teams[0].Role != "member" {
		t.Fatalf("GET teams = %+v, want one developers/member team", teams)
	}

	// A plain project member cannot associate a team (needs ADMINISTER_PROJECTS).
	resForbidden := doJSON(t, srv, http.MethodPost, bob, "/rest/api/3/project/TM/teams", map[string]any{
		"groupId": g.ID, "role": "admin",
	})
	if resForbidden.StatusCode != http.StatusForbidden {
		t.Fatalf("POST /project/TM/teams (member bob) status = %d, want 403", resForbidden.StatusCode)
	}
}
