package project

import (
	"testing"

	"github.com/google/uuid"
)

func TestCreateAndAcceptInvite(t *testing.T) {
	db := setupTestDB(t)
	db.AutoMigrate(&Invite{})
	projectID := uuid.New().String()
	userID := uuid.New().String()
	inv, err := CreateInvite(db, projectID, "invited@test.com", RoleMember)
	if err != nil {
		t.Fatalf("CreateInvite() error = %v", err)
	}
	if inv.Token == "" {
		t.Error("token empty")
	}
	pm, err := AcceptInvite(db, inv.Token, userID)
	if err != nil {
		t.Fatalf("AcceptInvite() error = %v", err)
	}
	if pm.ProjectID != projectID {
		t.Errorf("ProjectID mismatch")
	}
}
