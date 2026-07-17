package version

import "time"

// Version is a project release/version. It maps to the pre-existing `versions`
// table (from migration 000001), extended by migration 000019 with start_date
// and archived. The single `issue.version_id` FK is dead — multi fix-versions
// go through the issue_versions pivot instead.
type Version struct {
	ID          string     `gorm:"primaryKey;type:text" json:"id"`
	ProjectID   string     `gorm:"type:text;not null;index" json:"project_id"`
	Name        string     `gorm:"type:text;not null" json:"name"`
	Description string     `gorm:"type:text;default:''" json:"description"`
	Released    bool       `gorm:"not null;default:false" json:"released"`
	Archived    bool       `gorm:"not null;default:false" json:"archived"`
	StartDate   *time.Time `json:"start_date,omitempty"`
	ReleaseDate *time.Time `json:"release_date,omitempty"`
	CreatedAt   time.Time  `gorm:"autoCreateTime" json:"created_at"`
}

func (Version) TableName() string { return "versions" }

// IssueVersion is the many-to-many pivot linking issues to fix versions
// (mirrors issue.IssueLabel).
type IssueVersion struct {
	IssueID   string `gorm:"primaryKey;type:text" json:"issue_id"`
	VersionID string `gorm:"primaryKey;type:text" json:"version_id"`
}

func (IssueVersion) TableName() string { return "issue_versions" }
