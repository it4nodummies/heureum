package project

import (
	"crypto/rand"
	"encoding/hex"
	"errors"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Invite struct {
	ID         string `gorm:"primaryKey;type:text" json:"id"`
	ProjectID  string `gorm:"type:text;not null" json:"project_id"`
	Email      string `gorm:"type:text;not null" json:"email"`
	Token      string `gorm:"type:text;uniqueIndex;not null" json:"token"`
	Role       string `gorm:"type:text;not null;default:'member'" json:"role"`
	Accepted   bool   `gorm:"default:false" json:"accepted"`
	AcceptedBy string `gorm:"type:text" json:"accepted_by,omitempty"`
}

func (Invite) TableName() string { return "project_invites" }

func CreateInvite(db *gorm.DB, projectID, email string, role MemberRole) (*Invite, error) {
	if projectID == "" || email == "" {
		return nil, errors.New("project_id and email are required")
	}
	b := make([]byte, 32)
	rand.Read(b)
	inv := &Invite{
		ID:        uuid.New().String(),
		ProjectID: projectID,
		Email:     email,
		Token:     hex.EncodeToString(b),
		Role:      string(role),
	}
	if err := db.Create(inv).Error; err != nil {
		return nil, err
	}
	return inv, nil
}

func AcceptInvite(db *gorm.DB, token, userID string) (*ProjectMember, error) {
	var inv Invite
	if err := db.Where("token = ? AND accepted = ?", token, false).First(&inv).Error; err != nil {
		return nil, errors.New("invalid or expired invite")
	}
	pm := &ProjectMember{ProjectID: inv.ProjectID, UserID: userID, Role: MemberRole(inv.Role)}
	if err := db.Create(pm).Error; err != nil {
		return nil, err
	}
	db.Model(&inv).Updates(map[string]interface{}{"accepted": true, "accepted_by": userID})
	return pm, nil
}
