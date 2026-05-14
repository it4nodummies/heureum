package project

import "time"

type Type string

const (
	TypeScrum    Type = "scrum"
	TypeKanban   Type = "kanban"
	TypeBusiness Type = "business"
)

type Project struct {
	ID              string    `gorm:"primaryKey;type:text" json:"id"`
	OrgID           *string   `gorm:"type:text" json:"org_id,omitempty"`
	Name            string    `gorm:"not null;type:text" json:"name"`
	Key             string    `gorm:"uniqueIndex;not null;type:text" json:"key"`
	Description     string    `gorm:"type:text;default:''" json:"description"`
	Type            Type      `gorm:"type:text;not null;default:'scrum'" json:"type"`
	LeadUserID      *string   `gorm:"type:text" json:"lead_user_id,omitempty"`
	DefaultAssignee string    `gorm:"type:text;default:'unassigned'" json:"default_assignee"`
	IconURL         string    `gorm:"type:text;default:''" json:"icon_url"`
	IsArchived      bool      `gorm:"default:false" json:"is_archived"`
	CreatedAt       time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt       time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

func (Project) TableName() string { return "projects" }
