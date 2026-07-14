package group

import "time"

type Group struct {
	ID        string    `gorm:"primaryKey;type:text" json:"id"`
	Name      string    `gorm:"type:text;not null;uniqueIndex" json:"name"`
	CreatedAt time.Time `gorm:"autoCreateTime" json:"created_at"`
}

type GroupMember struct {
	GroupID string `gorm:"primaryKey;type:text" json:"group_id"`
	UserID  string `gorm:"primaryKey;type:text" json:"user_id"`
}

func (GroupMember) TableName() string { return "group_members" }
