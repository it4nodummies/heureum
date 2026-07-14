package board

import "time"

// Board è una board Agile legata a un progetto. SeqID è l'id pubblico intero
// esposto dall'API agile (l'UUID resta PK interna, come per progetti/issue).
type Board struct {
	ID        string    `gorm:"primaryKey;type:text" json:"id"`
	SeqID     int64     `gorm:"column:seq_id;uniqueIndex" json:"seq_id"`
	Name      string    `gorm:"type:text;not null" json:"name"`
	Type      string    `gorm:"type:text;not null;default:'scrum'" json:"type"`
	ProjectID string    `gorm:"type:text;not null;index" json:"project_id"`
	FilterID  *string   `gorm:"type:text" json:"filter_id,omitempty"`
	CreatedAt time.Time `gorm:"autoCreateTime" json:"created_at"`
}
