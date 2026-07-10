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
	SeqID           int64     `gorm:"column:seq_id;uniqueIndex" json:"seq_id"`
	OrgID           *string   `gorm:"type:text" json:"org_id,omitempty"`
	Name            string    `gorm:"not null;type:text" json:"name"`
	Key             string    `gorm:"uniqueIndex;not null;type:text" json:"key"`
	Description     string    `gorm:"type:text;default:''" json:"description"`
	Type            Type      `gorm:"type:text;not null;default:'scrum'" json:"type"`
	LeadUserID      *string   `gorm:"type:text" json:"lead_user_id,omitempty"`
	DefaultAssignee string    `gorm:"type:text;default:'unassigned'" json:"default_assignee"`
	IconURL         string    `gorm:"type:text;default:''" json:"icon_url"`
	CategoryID      *string   `gorm:"type:text" json:"category_id,omitempty"`
	AssigneeType    string    `gorm:"type:text;not null;default:'UNASSIGNED'" json:"assignee_type"`
	IsPrivate       bool      `gorm:"not null;default:false" json:"is_private"`
	Simplified      bool      `gorm:"not null;default:false" json:"simplified"`
	Style           string    `gorm:"type:text;not null;default:'classic'" json:"style"`
	URL             string    `gorm:"type:text;not null;default:''" json:"url"`
	IsArchived      bool      `gorm:"default:false" json:"is_archived"`
	CreatedAt       time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt       time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

func (Project) TableName() string { return "projects" }

// ProjectFavorite represents a user's starred project.
type ProjectFavorite struct {
	UserID    string    `gorm:"primaryKey;type:text" json:"user_id"`
	ProjectID string    `gorm:"primaryKey;type:text" json:"project_id"`
	CreatedAt time.Time `gorm:"autoCreateTime" json:"created_at"`
}

func (ProjectFavorite) TableName() string { return "project_favorites" }

// LeadInfo is the minimal user info embedded in project responses.
type LeadInfo struct {
	ID          string `json:"id"`
	DisplayName string `json:"display_name"`
	AvatarURL   string `json:"avatar_url"`
	Email       string `json:"email"`
}

// ProjectWithLead is the enriched project response including lead user and starred status.
type ProjectWithLead struct {
	Project
	Lead      *LeadInfo `json:"lead,omitempty"`
	IsStarred bool      `json:"is_starred"`
}

// ListFilter holds parameters for filtering/sorting the project list.
type ListFilter struct {
	Search     string
	Types      []string
	SortKey    string // "name" | "key" | "type" | "created_at"
	SortDir    string // "asc" | "desc"
	StartAt    int
	MaxResults int
}

// ProjectCategory rispecchia la tabella project_categories (migrazione 000002).
type ProjectCategory struct {
	ID          string `gorm:"primaryKey;type:text" json:"id"`
	Name        string `gorm:"type:text;not null" json:"name"`
	Description string `gorm:"type:text;default:''" json:"description"`
}

func (ProjectCategory) TableName() string { return "project_categories" }

func TemplateKeyForType(t Type) string {
	switch t {
	case TypeKanban:
		return "com.pyxis.greenhopper.jira:gh-kanban-template"
	case TypeBusiness:
		return "com.atlassian.jira-core-project-templates:jira-core-simplified-process-control"
	default:
		return "com.pyxis.greenhopper.jira:gh-scrum-template"
	}
}

func TypeForTemplateKey(k string) Type {
	switch k {
	case "com.pyxis.greenhopper.jira:gh-kanban-template":
		return TypeKanban
	case "com.atlassian.jira-core-project-templates:jira-core-simplified-process-control":
		return TypeBusiness
	default:
		return TypeScrum
	}
}

func ProjectTypeKeyForType(t Type) string {
	if t == TypeBusiness {
		return "business"
	}
	return "software"
}
