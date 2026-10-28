package project

import (
	"testing"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/it4nodummies/heureum/internal/storetest"
)

// createInviteTestProject inserts a minimal, valid projects row.
// project_invites.project_id is a NOT NULL FK to projects(id) in the real
// schema, so the parent must actually exist (mirrors the issue package helper).
func createInviteTestProject(t *testing.T, db *gorm.DB, id string) {
	t.Helper()
	if err := db.Exec(
		`INSERT INTO projects (id, name, key) VALUES (?, ?, ?) ON CONFLICT (id) DO NOTHING`,
		id, id, id,
	).Error; err != nil {
		t.Fatalf("createInviteTestProject(%q): %v", id, err)
	}
}

// createInviteTestUser inserts a minimal, valid users row.
// project_invites.accepted_by and project_members.user_id reference users(id).
func createInviteTestUser(t *testing.T, db *gorm.DB, id string) {
	t.Helper()
	if err := db.Exec(
		`INSERT INTO users (id, email, username) VALUES (?, ?, ?) ON CONFLICT (id) DO NOTHING`,
		id, id+"@example.test", id,
	).Error; err != nil {
		t.Fatalf("createInviteTestUser(%q): %v", id, err)
	}
}

// acceptedByOf reads accepted_by through raw SQL so the assertion does not
// depend on how the Go model maps the column.
func acceptedByOf(t *testing.T, db *gorm.DB, inviteID string) *string {
	t.Helper()
	var row struct{ AcceptedBy *string }
	if err := db.Raw(`SELECT accepted_by FROM project_invites WHERE id = ?`, inviteID).Scan(&row).Error; err != nil {
		t.Fatalf("read accepted_by: %v", err)
	}
	return row.AcceptedBy
}

func TestCreateInvite_PersistsOnRealSchema(t *testing.T) {
	db := storetest.New(t)
	projectID := uuid.New().String()
	createInviteTestProject(t, db, projectID)

	inv, err := CreateInvite(db, projectID, "invited@test.com", RoleMember)
	if err != nil {
		t.Fatalf("CreateInvite() error = %v", err)
	}
	if inv.Token == "" {
		t.Error("token empty")
	}

	var got Invite
	if err := db.First(&got, "id = ?", inv.ID).Error; err != nil {
		t.Fatalf("invite row not readable: %v", err)
	}
	if got.Accepted {
		t.Error("new invite must not be accepted")
	}
	if ab := acceptedByOf(t, db, inv.ID); ab != nil {
		t.Errorf("accepted_by = %q, want NULL until accepted", *ab)
	}
}

func TestAcceptInvite_SetsAcceptedBy(t *testing.T) {
	db := storetest.New(t)
	projectID := uuid.New().String()
	userID := uuid.New().String()
	createInviteTestProject(t, db, projectID)
	createInviteTestUser(t, db, userID)

	inv, err := CreateInvite(db, projectID, "invited@test.com", RoleMember)
	if err != nil {
		t.Fatalf("CreateInvite() error = %v", err)
	}

	pm, err := AcceptInvite(db, inv.Token, userID)
	if err != nil {
		t.Fatalf("AcceptInvite() error = %v", err)
	}
	if pm.ProjectID != projectID || pm.UserID != userID || pm.Role != RoleMember {
		t.Errorf("member = %+v, want project %s user %s role %s", pm, projectID, userID, RoleMember)
	}

	var got Invite
	if err := db.First(&got, "id = ?", inv.ID).Error; err != nil {
		t.Fatalf("invite row not readable: %v", err)
	}
	if !got.Accepted {
		t.Error("invite not marked accepted")
	}
	ab := acceptedByOf(t, db, inv.ID)
	if ab == nil || *ab != userID {
		t.Errorf("accepted_by = %v, want %q", ab, userID)
	}

	// An accepted invite cannot be used again.
	if _, err := AcceptInvite(db, inv.Token, userID); err == nil {
		t.Error("second AcceptInvite() with the same token should fail")
	}
}
