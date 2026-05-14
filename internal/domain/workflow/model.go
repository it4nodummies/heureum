package workflow

import "time"

type StatusCategory string

const (
	CategoryTodo       StatusCategory = "todo"
	CategoryInProgress StatusCategory = "inprogress"
	CategoryDone       StatusCategory = "done"
)

type Workflow struct {
	ID        string           `gorm:"primaryKey;type:text" json:"id"`
	ProjectID string           `gorm:"type:text;not null" json:"project_id"`
	Name      string           `gorm:"type:text;not null" json:"name"`
	Statuses  []WorkflowStatus `gorm:"foreignKey:WorkflowID" json:"statuses,omitempty"`
	CreatedAt time.Time        `gorm:"autoCreateTime" json:"created_at"`
}

type WorkflowStatus struct {
	ID         string         `gorm:"primaryKey;type:text" json:"id"`
	WorkflowID string         `gorm:"type:text;not null;index" json:"workflow_id"`
	Name       string         `gorm:"type:text;not null" json:"name"`
	Category   StatusCategory `gorm:"type:text;not null;default:'inprogress'" json:"category"`
	Color      string         `gorm:"type:text;default:'#6B7280'" json:"color"`
	Position   int            `gorm:"not null;default:0" json:"position"`
}

type WorkflowTransition struct {
	ID           string `gorm:"primaryKey;type:text" json:"id"`
	WorkflowID   string `gorm:"type:text;not null;index" json:"workflow_id"`
	FromStatusID string `gorm:"type:text;not null" json:"from_status_id"`
	ToStatusID   string `gorm:"type:text;not null" json:"to_status_id"`
}
