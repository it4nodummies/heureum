package webhook

import (
	"encoding/json"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Events deserializza EventsJSON in slice (vuoto se non valido).
func (w *Webhook) Events() []string {
	var out []string
	if w.EventsJSON == "" {
		return out
	}
	_ = json.Unmarshal([]byte(w.EventsJSON), &out)
	return out
}

type Service struct{ db *gorm.DB }

func NewService(db *gorm.DB) *Service { return &Service{db: db} }

func (s *Service) Create(projectID, url, secret string, events []string) (*Webhook, error) {
	ev, _ := json.Marshal(events)
	h := &Webhook{ID: uuid.NewString(), ProjectID: projectID, URL: url, Secret: secret, EventsJSON: string(ev), IsActive: true}
	if err := s.db.Create(h).Error; err != nil {
		return nil, err
	}
	return h, nil
}

func (s *Service) ListByProject(projectID string) ([]Webhook, error) {
	var hooks []Webhook
	if err := s.db.Where("project_id = ?", projectID).Order("created_at DESC").Find(&hooks).Error; err != nil {
		return nil, err
	}
	return hooks, nil
}

func (s *Service) Delete(id string) error {
	return s.db.Where("id = ?", id).Delete(&Webhook{}).Error
}

// ListActiveForEvent restituisce i webhook attivi del progetto sottoscritti a eventType.
func (s *Service) ListActiveForEvent(projectID, eventType string) ([]Webhook, error) {
	var hooks []Webhook
	if err := s.db.Where("project_id = ? AND is_active = ?", projectID, true).Find(&hooks).Error; err != nil {
		return nil, err
	}
	out := make([]Webhook, 0, len(hooks))
	for _, h := range hooks {
		for _, e := range h.Events() {
			if e == eventType {
				out = append(out, h)
				break
			}
		}
	}
	return out, nil
}

func (s *Service) RecordDelivery(webhookID, eventType, url string, statusCode int, success bool, errMsg string) error {
	d := &Delivery{ID: uuid.NewString(), WebhookID: webhookID, EventType: eventType, URL: url, StatusCode: statusCode, Success: success, Error: errMsg}
	return s.db.Create(d).Error
}
