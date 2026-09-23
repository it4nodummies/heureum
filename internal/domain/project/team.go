package project

import "time"

// ProjectTeam associa un gruppo (team) a un progetto con un ruolo.
type ProjectTeam struct {
	ProjectID string     `gorm:"primaryKey;type:text" json:"project_id"`
	GroupID   string     `gorm:"primaryKey;type:text" json:"group_id"`
	Role      MemberRole `gorm:"type:text;not null;default:'member'" json:"role"`
	CreatedAt time.Time  `gorm:"autoCreateTime" json:"created_at"`
}

func (ProjectTeam) TableName() string { return "project_teams" }

// ProjectTeamInfo è la proiezione con il nome del gruppo per l'API/UI.
type ProjectTeamInfo struct {
	GroupID   string     `json:"groupId"`
	GroupName string     `json:"name"`
	Role      MemberRole `json:"role"`
}
