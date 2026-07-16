package customfield

import "time"

type FieldType string

const (
	FieldTypeText        FieldType = "text"
	FieldTypeNumber      FieldType = "number"
	FieldTypeDate        FieldType = "date"
	FieldTypeSelect      FieldType = "select"
	FieldTypeMultiSelect FieldType = "multiselect"
	FieldTypeUser        FieldType = "user"
)

type CustomField struct {
	ID        string    `gorm:"primaryKey;type:text" json:"id"`
	ProjectID string    `gorm:"type:text;not null;index" json:"project_id"`
	Name      string    `gorm:"type:text;not null" json:"name"`
	FieldType FieldType `gorm:"type:text;not null" json:"field_type"`
	Required  bool      `gorm:"not null;default:false" json:"required"`
	CreatedAt time.Time `gorm:"autoCreateTime" json:"created_at"`
}

type CustomFieldOption struct {
	ID       string `gorm:"primaryKey;type:text" json:"id"`
	FieldID  string `gorm:"type:text;not null;index" json:"field_id"`
	Value    string `gorm:"type:text;not null" json:"value"`
	Position int    `gorm:"not null;default:0" json:"position"`
}

type IssueCustomValue struct {
	IssueID     string     `gorm:"primaryKey;type:text" json:"issue_id"`
	FieldID     string     `gorm:"primaryKey;type:text" json:"field_id"`
	ValueText   string     `gorm:"type:text;default:''" json:"value_text"`
	ValueNumber *float64   `json:"value_number,omitempty"`
	ValueDate   *time.Time `json:"value_date,omitempty"`
	OptionID    *string    `gorm:"type:text" json:"option_id,omitempty"`
}
