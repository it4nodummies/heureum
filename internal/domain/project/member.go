package project

type MemberRole string

const (
	RoleAdmin  MemberRole = "admin"
	RoleMember MemberRole = "member"
	RoleViewer MemberRole = "viewer"
)

type ProjectMember struct {
	ProjectID string     `gorm:"primaryKey;type:text;not null" json:"project_id"`
	UserID    string     `gorm:"primaryKey;type:text;not null" json:"user_id"`
	Role      MemberRole `gorm:"type:text;not null;default:'member'" json:"role"`
}

func (ProjectMember) TableName() string { return "project_members" }
