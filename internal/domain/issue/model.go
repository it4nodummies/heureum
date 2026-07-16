package issue

import (
	"time"
)

type Priority string

const (
	PriorityHighest Priority = "highest"
	PriorityHigh    Priority = "high"
	PriorityMedium  Priority = "medium"
	PriorityLow     Priority = "low"
	PriorityLowest  Priority = "lowest"
)

type IssueType struct {
	ID          string `gorm:"primaryKey;type:text" json:"id"`
	ProjectID   string `gorm:"type:text;not null;index" json:"project_id"`
	Name        string `gorm:"type:text;not null" json:"name"`
	Description string `gorm:"type:text;default:''" json:"description"`
	Icon        string `gorm:"type:text;default:'task'" json:"icon"`
	Color       string `gorm:"type:text;default:'#6B7280'" json:"color"`
	IsSubtask   bool   `gorm:"default:false" json:"is_subtask"`
}

type Issue struct {
	ID                string     `gorm:"primaryKey;type:text" json:"id"`
	ProjectID         string     `gorm:"type:text;not null;index" json:"project_id"`
	Key               string     `gorm:"uniqueIndex;not null;type:text" json:"key"`
	Title             string     `gorm:"type:text;not null" json:"title"`
	DescriptionJSON   string     `gorm:"type:text;default:'{}'" json:"description_json"`
	TypeID            *string    `gorm:"type:text" json:"type_id,omitempty"`
	StatusID          *string    `gorm:"type:text" json:"status_id,omitempty"`
	Priority          Priority   `gorm:"type:text;default:'medium'" json:"priority"`
	AssigneeID        *string    `gorm:"type:text" json:"assignee_id,omitempty"`
	ReporterID        *string    `gorm:"type:text" json:"reporter_id,omitempty"`
	ResolutionID      *string    `gorm:"type:text" json:"resolution_id,omitempty"`
	ParentID          *string    `gorm:"type:text;index" json:"parent_id,omitempty"`
	SprintID          *string    `gorm:"type:text;index" json:"sprint_id,omitempty"`
	VersionID         *string    `gorm:"type:text" json:"version_id,omitempty"`
	StoryPoints       int        `gorm:"default:0" json:"story_points"`
	OriginalEstimate  int        `gorm:"default:0" json:"original_estimate"`
	RemainingEstimate int        `gorm:"column:remaining_estimate;default:0" json:"remaining_estimate"`
	TimeSpent         int        `gorm:"default:0" json:"time_spent"`
	StartDate         *time.Time `json:"start_date,omitempty"`
	DueDate           *time.Time `json:"due_date,omitempty"`
	Environment       string     `gorm:"type:text;default:''" json:"environment"`
	IsArchived        bool       `gorm:"default:false" json:"is_archived"`
	Position          float64    `gorm:"not null;default:0" json:"position"`
	SeqID             int64      `gorm:"column:seq_id;uniqueIndex" json:"seq_id"`
	CreatedAt         time.Time  `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt         time.Time  `gorm:"autoUpdateTime" json:"updated_at"`
	Type              *IssueType `gorm:"foreignKey:TypeID" json:"-"`
}

type Label struct {
	ID        string `gorm:"primaryKey;type:text" json:"id"`
	ProjectID string `gorm:"type:text;not null;uniqueIndex:idx_project_label" json:"project_id"`
	Name      string `gorm:"type:text;not null;uniqueIndex:idx_project_label" json:"name"`
	Color     string `gorm:"type:text;default:'#6B7280'" json:"color"`
}

type IssueLabel struct {
	IssueID string `gorm:"primaryKey;type:text" json:"issue_id"`
	LabelID string `gorm:"primaryKey;type:text" json:"label_id"`
}

type LinkType string

const (
	LinkBlocks     LinkType = "blocks"
	LinkIsBlocked  LinkType = "is_blocked"
	LinkDuplicates LinkType = "duplicates"
	LinkRelates    LinkType = "relates"
)

type IssueLink struct {
	ID       string   `gorm:"primaryKey;type:text" json:"id"`
	SourceID string   `gorm:"type:text;not null;index" json:"source_id"`
	TargetID string   `gorm:"type:text;not null;index" json:"target_id"`
	LinkType LinkType `gorm:"type:text;not null" json:"link_type"`
}

type IssueHistory struct {
	ID        string    `gorm:"primaryKey;type:text" json:"id"`
	IssueID   string    `gorm:"type:text;not null;index" json:"issue_id"`
	ActorID   *string   `gorm:"type:text" json:"actor_id,omitempty"`
	FieldName string    `gorm:"type:text;not null" json:"field_name"`
	OldValue  string    `gorm:"type:text;default:''" json:"old_value"`
	NewValue  string    `gorm:"type:text;default:''" json:"new_value"`
	CreatedAt time.Time `gorm:"autoCreateTime" json:"created_at"`
}

func (IssueHistory) TableName() string { return "issue_history" }

type Comment struct {
	ID        string    `gorm:"primaryKey;type:text" json:"id"`
	IssueID   string    `gorm:"type:text;not null;index" json:"issue_id"`
	AuthorID  *string   `gorm:"type:text" json:"author_id,omitempty"`
	BodyJSON  string    `gorm:"type:text;default:'{}'" json:"body_json"`
	IsDeleted bool      `gorm:"default:false" json:"is_deleted"`
	CreatedAt time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

type IssueWatcher struct {
	IssueID string `gorm:"primaryKey;type:text" json:"issue_id"`
	UserID  string `gorm:"primaryKey;type:text" json:"user_id"`
}

type IssueAttachment struct {
	ID         string    `gorm:"primaryKey;type:text" json:"id"`
	IssueID    string    `gorm:"type:text;not null;index" json:"issue_id"`
	Filename   string    `gorm:"type:text;not null" json:"filename"`
	FilePath   string    `gorm:"type:text;not null" json:"file_path"`
	FileSize   int64     `gorm:"not null;default:0" json:"file_size"`
	UploaderID *string   `gorm:"type:text" json:"uploader_id,omitempty"`
	CreatedAt  time.Time `gorm:"autoCreateTime" json:"created_at"`
}
