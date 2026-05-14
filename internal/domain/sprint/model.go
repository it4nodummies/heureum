package sprint

import "time"

type State string

const (
	StateActive State = "active"
	StateClosed State = "closed"
	StateFuture State = "future"
)

type Sprint struct {
	ID        string     `gorm:"primaryKey;type:text" json:"id"`
	ProjectID string     `gorm:"type:text;not null;index" json:"project_id"`
	Name      string     `gorm:"type:text;not null" json:"name"`
	Goal      string     `gorm:"type:text;default:''" json:"goal"`
	State     State      `gorm:"type:text;default:'future'" json:"state"`
	StartDate *time.Time `json:"start_date,omitempty"`
	EndDate   *time.Time `json:"end_date,omitempty"`
	CreatedAt time.Time  `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt time.Time  `gorm:"autoUpdateTime" json:"updated_at"`
}
