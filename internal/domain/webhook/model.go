package webhook

import "time"

// Webhook è una registrazione webhook per-progetto (estensione: non il modello
// dynamic-webhook Connect/OAuth di Jira). events è un JSON array di stringhe.
type Webhook struct {
	ID         string    `gorm:"primaryKey;type:text" json:"id"`
	ProjectID  string    `gorm:"column:project_id;type:text;not null;index" json:"project_id"`
	URL        string    `gorm:"column:url;type:text;not null" json:"url"`
	Secret     string    `gorm:"column:secret;type:text;default:''" json:"secret,omitempty"`
	EventsJSON string    `gorm:"column:events_json;type:text;default:'[]'" json:"-"`
	IsActive   bool      `gorm:"column:is_active;default:true" json:"is_active"`
	CreatedAt  time.Time `gorm:"autoCreateTime" json:"created_at"`
}

func (Webhook) TableName() string { return "webhooks" }

// Delivery è il log di un tentativo di consegna.
type Delivery struct {
	ID         string    `gorm:"primaryKey;type:text" json:"id"`
	WebhookID  string    `gorm:"column:webhook_id;type:text;not null;index" json:"webhook_id"`
	EventType  string    `gorm:"column:event_type;type:text;not null" json:"event_type"`
	URL        string    `gorm:"column:url;type:text;not null" json:"url"`
	StatusCode int       `gorm:"column:status_code" json:"status_code"`
	Success    bool      `gorm:"column:success" json:"success"`
	Error      string    `gorm:"column:error;type:text;default:''" json:"error,omitempty"`
	CreatedAt  time.Time `gorm:"autoCreateTime" json:"created_at"`
}

func (Delivery) TableName() string { return "webhook_deliveries" }
