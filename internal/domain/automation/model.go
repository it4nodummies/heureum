package automation

import "time"

type TriggerType string

const (
	TriggerIssueCreated      TriggerType = "issue_created"
	TriggerIssueUpdated      TriggerType = "issue_updated"
	TriggerIssueTransitioned TriggerType = "issue_transitioned"
)

type AutomationRule struct {
	ID             string    `gorm:"primaryKey;type:text" json:"id"`
	ProjectID      string    `gorm:"type:text;not null;index" json:"project_id"`
	Name           string    `gorm:"type:text;not null" json:"name"`
	IsActive       bool      `gorm:"default:true" json:"is_active"`
	TriggerType    string    `gorm:"type:text;not null" json:"trigger_type"`
	ConditionsJSON string    `gorm:"type:text;default:'{}'" json:"conditions_json"`
	ActionsJSON    string    `gorm:"type:text;default:'[]'" json:"actions_json"`
	CreatedAt      time.Time `gorm:"autoCreateTime" json:"created_at"`
}

type AutomationRun struct {
	ID          string    `gorm:"primaryKey;type:text" json:"id"`
	RuleID      string    `gorm:"type:text;not null;index" json:"rule_id"`
	IssueID     *string   `gorm:"type:text" json:"issue_id,omitempty"`
	TriggeredAt time.Time `gorm:"autoCreateTime" json:"triggered_at"`
	Status      string    `gorm:"type:text;default:'success'" json:"status"`
	Log         string    `gorm:"type:text;default:''" json:"log"`
}
